#include "ASTExtractor.h"
#include "clang/AST/DeclCXX.h"
#include "clang/AST/DeclTemplate.h"
#include "clang/AST/Expr.h"
#include "clang/AST/ExprCXX.h"
#include "clang/AST/Type.h"
#include "llvm/Support/raw_ostream.h"
#include <algorithm>
#include <sstream>

ASTExtractor::ASTExtractor(clang::ASTContext &Ctx, JSONLWriter &Writer,
                           const std::string &RepoRoot, bool Verbose)
    : Ctx(Ctx), SM(Ctx.getSourceManager()), Writer(Writer),
      RepoRoot(RepoRoot), Verbose(Verbose) {
    std::replace(this->RepoRoot.begin(), this->RepoRoot.end(), '\\', '/');
    if (!this->RepoRoot.empty() && this->RepoRoot.back() != '/')
        this->RepoRoot += '/';
    std::transform(this->RepoRoot.begin(), this->RepoRoot.end(),
                   this->RepoRoot.begin(), ::tolower);
}

ASTExtractor::LocationInfo ASTExtractor::getLocation(clang::SourceLocation loc) {
    LocationInfo info{};
    if (loc.isInvalid()) return info;

    clang::SourceLocation expansionLoc = SM.getExpansionLoc(loc);
    if (expansionLoc.isInvalid()) return info;

    std::string path = SM.getFilename(expansionLoc).str();
    if (path.empty() || !isInsideRepo(path)) return info;

    info.RelPath = makeRelative(path);
    info.StartLine = SM.getExpansionLineNumber(expansionLoc);
    info.EndLine = info.StartLine;
    info.Valid = true;
    return info;
}

ASTExtractor::LocationInfo ASTExtractor::getRange(clang::SourceRange range) {
    auto info = getLocation(range.getBegin());
    if (!info.Valid) return info;

    clang::SourceLocation endLoc = SM.getExpansionLoc(range.getEnd());
    if (endLoc.isValid())
        info.EndLine = SM.getExpansionLineNumber(endLoc);
    return info;
}

std::string ASTExtractor::makeRelative(const std::string &absPath) {
    std::string normalized = absPath;
    std::replace(normalized.begin(), normalized.end(), '\\', '/');

    std::string lower = normalized;
    std::transform(lower.begin(), lower.end(), lower.begin(), ::tolower);

    if (lower.size() >= RepoRoot.size() &&
        lower.substr(0, RepoRoot.size()) == RepoRoot)
        return normalized.substr(RepoRoot.size());

    return normalized;
}

bool ASTExtractor::isInsideRepo(const std::string &absPath) {
    std::string normalized = absPath;
    std::replace(normalized.begin(), normalized.end(), '\\', '/');
    std::transform(normalized.begin(), normalized.end(), normalized.begin(), ::tolower);
    return normalized.size() >= RepoRoot.size() &&
           normalized.substr(0, RepoRoot.size()) == RepoRoot;
}

std::string ASTExtractor::getFQN(const clang::NamedDecl *D) {
    std::string fqn;
    llvm::raw_string_ostream os(fqn);
    D->printQualifiedName(os);
    return fqn;
}

std::string ASTExtractor::getSignature(const clang::FunctionDecl *D) {
    std::string sig = "(";
    for (unsigned i = 0; i < D->getNumParams(); ++i) {
        if (i > 0) sig += ",";
        sig += D->getParamDecl(i)->getType().getAsString();
    }
    sig += ")";
    return sig;
}

std::string ASTExtractor::makeNodeID(const std::string &label, const std::string &file,
                                     const std::string &fqn, const std::string &sig) {
    return label + ":" + file + ":" + fqn + ":" + sig;
}

std::string ASTExtractor::getEnclosingFunctionID() {
    if (FunctionStack.empty()) return "";
    return FunctionStack.back().NodeID;
}

bool ASTExtractor::VisitFunctionDecl(clang::FunctionDecl *D) {
    if (!D->isThisDeclarationADefinition())
        return true;

    // Skip implicit and compiler-generated
    if (D->isImplicit())
        return true;

    auto locInfo = getRange(D->getSourceRange());
    if (!locInfo.Valid) return true;

    std::string fqn = getFQN(D);
    std::string sig = getSignature(D);

    // Determine label
    std::string label = "Function";
    if (auto *CD = llvm::dyn_cast<clang::CXXConstructorDecl>(D))
        label = "Constructor";

    std::string nodeID = makeNodeID(label, locInfo.RelPath, fqn, sig);

    if (EmittedNodes.count(nodeID))
        return true;
    EmittedNodes.insert(nodeID);

    // Extract namespace
    std::string ns;
    const clang::DeclContext *dc = D->getDeclContext();
    while (dc) {
        if (auto *nsDecl = llvm::dyn_cast<clang::NamespaceDecl>(dc)) {
            if (ns.empty())
                ns = nsDecl->getNameAsString();
            else
                ns = nsDecl->getNameAsString() + "::" + ns;
        }
        dc = dc->getParent();
    }

    std::vector<std::pair<std::string, std::string>> props = {
        {"name", D->getNameAsString()},
        {"fqn", fqn},
        {"file", locInfo.RelPath},
        {"start_line", std::to_string(locInfo.StartLine)},
        {"end_line", std::to_string(locInfo.EndLine)},
    };
    if (!ns.empty())
        props.push_back({"namespace", ns});

    Writer.emitNode(nodeID, label, props);

    // DEFINED_IN edge
    std::string fileID = "File:" + locInfo.RelPath;
    Writer.emitEdge(nodeID, fileID, "DEFINED_IN");

    // HAS_METHOD edge if this is a class method
    if (auto *MD = llvm::dyn_cast<clang::CXXMethodDecl>(D)) {
        if (auto *RD = MD->getParent()) {
            if (RD->isThisDeclarationADefinition()) {
                auto classLoc = getLocation(RD->getLocation());
                if (classLoc.Valid) {
                    std::string classFQN = getFQN(RD);
                    std::string classID = makeNodeID("Class", classLoc.RelPath, classFQN, "");
                    std::string edgeKey = classID + " -HAS_METHOD-> " + nodeID;
                    if (!EmittedEdges.count(edgeKey)) {
                        EmittedEdges.insert(edgeKey);
                        Writer.emitEdge(classID, nodeID, "HAS_METHOD");
                    }
                }
            }
        }
    }

    return true;
}

// Traverse methods manage the FunctionStack so that VisitCallExpr/VisitDeclRefExpr
// know which function they're inside. We override Traverse (not Visit) because
// Visit doesn't have a corresponding "leave" callback.
static std::string computeFuncNodeID(ASTExtractor &E, clang::FunctionDecl *D);

bool ASTExtractor::TraverseFunctionDecl(clang::FunctionDecl *D) {
    if (D->isThisDeclarationADefinition() && !D->isImplicit()) {
        auto locInfo = getRange(D->getSourceRange());
        if (locInfo.Valid) {
            std::string fqn = getFQN(D);
            std::string sig = getSignature(D);
            std::string label = "Function";
            std::string nodeID = makeNodeID(label, locInfo.RelPath, fqn, sig);
            FunctionStack.push_back({nodeID, fqn});
            bool ret = RecursiveASTVisitor::TraverseFunctionDecl(D);
            FunctionStack.pop_back();
            return ret;
        }
    }
    return RecursiveASTVisitor::TraverseFunctionDecl(D);
}

bool ASTExtractor::TraverseCXXMethodDecl(clang::CXXMethodDecl *D) {
    if (D->isThisDeclarationADefinition() && !D->isImplicit()) {
        auto locInfo = getRange(D->getSourceRange());
        if (locInfo.Valid) {
            std::string fqn = getFQN(D);
            std::string sig = getSignature(D);
            std::string label = "Function";
            std::string nodeID = makeNodeID(label, locInfo.RelPath, fqn, sig);
            FunctionStack.push_back({nodeID, fqn});
            bool ret = RecursiveASTVisitor::TraverseCXXMethodDecl(D);
            FunctionStack.pop_back();
            return ret;
        }
    }
    return RecursiveASTVisitor::TraverseCXXMethodDecl(D);
}

bool ASTExtractor::TraverseCXXConstructorDecl(clang::CXXConstructorDecl *D) {
    if (D->isThisDeclarationADefinition() && !D->isImplicit()) {
        auto locInfo = getRange(D->getSourceRange());
        if (locInfo.Valid) {
            std::string fqn = getFQN(D);
            std::string sig = getSignature(D);
            std::string label = "Constructor";
            std::string nodeID = makeNodeID(label, locInfo.RelPath, fqn, sig);
            FunctionStack.push_back({nodeID, fqn});
            bool ret = RecursiveASTVisitor::TraverseCXXConstructorDecl(D);
            FunctionStack.pop_back();
            return ret;
        }
    }
    return RecursiveASTVisitor::TraverseCXXConstructorDecl(D);
}

bool ASTExtractor::TraverseCXXDestructorDecl(clang::CXXDestructorDecl *D) {
    if (D->isThisDeclarationADefinition() && !D->isImplicit()) {
        auto locInfo = getRange(D->getSourceRange());
        if (locInfo.Valid) {
            std::string fqn = getFQN(D);
            std::string sig = getSignature(D);
            std::string label = "Function";
            std::string nodeID = makeNodeID(label, locInfo.RelPath, fqn, sig);
            FunctionStack.push_back({nodeID, fqn});
            bool ret = RecursiveASTVisitor::TraverseCXXDestructorDecl(D);
            FunctionStack.pop_back();
            return ret;
        }
    }
    return RecursiveASTVisitor::TraverseCXXDestructorDecl(D);
}

bool ASTExtractor::VisitCXXRecordDecl(clang::CXXRecordDecl *D) {
    if (!D->isThisDeclarationADefinition())
        return true;
    if (D->isImplicit() || D->isLambda())
        return true;

    auto locInfo = getRange(D->getSourceRange());
    if (!locInfo.Valid) return true;

    std::string fqn = getFQN(D);
    std::string nodeID = makeNodeID("Class", locInfo.RelPath, fqn, "");

    if (EmittedNodes.count(nodeID))
        return true;
    EmittedNodes.insert(nodeID);

    std::vector<std::pair<std::string, std::string>> props = {
        {"name", D->getNameAsString()},
        {"fqn", fqn},
        {"file", locInfo.RelPath},
        {"start_line", std::to_string(locInfo.StartLine)},
        {"end_line", std::to_string(locInfo.EndLine)},
    };

    Writer.emitNode(nodeID, "Class", props);

    // DEFINED_IN edge
    std::string fileID = "File:" + locInfo.RelPath;
    Writer.emitEdge(nodeID, fileID, "DEFINED_IN");

    // INHERITS edges
    if (D->hasDefinition()) {
        for (const auto &base : D->bases()) {
            const clang::Type *baseType = base.getType().getTypePtr();
            if (auto *baseRD = baseType->getAsCXXRecordDecl()) {
                if (baseRD->isImplicit()) continue;

                std::string baseFQN = getFQN(baseRD);
                auto baseLoc = getLocation(baseRD->getLocation());
                std::string baseFile = baseLoc.Valid ? baseLoc.RelPath : "";
                std::string baseID = makeNodeID("Class", baseFile, baseFQN, "");

                // Emit base class node if inside repo
                if (baseLoc.Valid && !EmittedNodes.count(baseID)) {
                    EmittedNodes.insert(baseID);
                    Writer.emitNode(baseID, "Class", {
                        {"name", baseRD->getNameAsString()},
                        {"fqn", baseFQN},
                        {"file", baseFile},
                        {"start_line", std::to_string(baseLoc.StartLine)},
                    });
                }

                std::string edgeKey = nodeID + " -INHERITS-> " + baseID;
                if (!EmittedEdges.count(edgeKey)) {
                    EmittedEdges.insert(edgeKey);
                    Writer.emitEdge(nodeID, baseID, "INHERITS");
                }
            }
        }
    }

    return true;
}

bool ASTExtractor::VisitVarDecl(clang::VarDecl *D) {
    // Only file-scope (global/namespace-scope) variables
    if (!D->isFileVarDecl())
        return true;
    if (D->isImplicit())
        return true;

    auto locInfo = getLocation(D->getLocation());
    if (!locInfo.Valid) return true;

    std::string fqn = getFQN(D);
    std::string nodeID = makeNodeID("Global", locInfo.RelPath, fqn, "");

    if (EmittedNodes.count(nodeID))
        return true;
    EmittedNodes.insert(nodeID);

    Writer.emitNode(nodeID, "Global", {
        {"name", D->getNameAsString()},
        {"fqn", fqn},
        {"file", locInfo.RelPath},
        {"start_line", std::to_string(locInfo.StartLine)},
        {"type", D->getType().getAsString()},
    });

    std::string fileID = "File:" + locInfo.RelPath;
    Writer.emitEdge(nodeID, fileID, "DEFINED_IN");

    return true;
}

bool ASTExtractor::VisitFieldDecl(clang::FieldDecl *D) {
    if (D->isImplicit())
        return true;

    auto locInfo = getLocation(D->getLocation());
    if (!locInfo.Valid) return true;

    auto *RD = llvm::dyn_cast<clang::RecordDecl>(D->getDeclContext());
    if (!RD) return true;

    std::string fqn = getFQN(D);
    std::string nodeID = makeNodeID("Field", locInfo.RelPath, fqn, "");

    if (EmittedNodes.count(nodeID))
        return true;
    EmittedNodes.insert(nodeID);

    Writer.emitNode(nodeID, "Field", {
        {"name", D->getNameAsString()},
        {"fqn", fqn},
        {"file", locInfo.RelPath},
        {"start_line", std::to_string(locInfo.StartLine)},
        {"type", D->getType().getAsString()},
    });

    // DEFINES edge from class to field
    auto *cxxRD = llvm::dyn_cast<clang::CXXRecordDecl>(RD);
    if (cxxRD) {
        auto classLoc = getLocation(cxxRD->getLocation());
        if (classLoc.Valid) {
            std::string classFQN = getFQN(cxxRD);
            std::string classID = makeNodeID("Class", classLoc.RelPath, classFQN, "");
            Writer.emitEdge(classID, nodeID, "DEFINES");
        }
    }

    // DEPENDS_ON edge for field type references
    const clang::Type *fieldType = D->getType().getTypePtr();
    if (auto *refRD = fieldType->getAsCXXRecordDecl()) {
        if (!refRD->isImplicit()) {
            auto refLoc = getLocation(refRD->getLocation());
            if (refLoc.Valid) {
                std::string refFQN = getFQN(refRD);
                std::string refID = makeNodeID("Class", refLoc.RelPath, refFQN, "");

                std::string ownerFQN = getFQN(RD);
                auto ownerLoc = getLocation(RD->getLocation());
                if (ownerLoc.Valid) {
                    std::string ownerID = makeNodeID("Class", ownerLoc.RelPath, ownerFQN, "");
                    std::string edgeKey = ownerID + " -DEPENDS_ON-> " + refID;
                    if (!EmittedEdges.count(edgeKey)) {
                        EmittedEdges.insert(edgeKey);
                        Writer.emitEdge(ownerID, refID, "DEPENDS_ON");
                    }
                }
            }
        }
    }

    return true;
}

bool ASTExtractor::VisitCallExpr(clang::CallExpr *E) {
    std::string callerID = getEnclosingFunctionID();
    if (callerID.empty()) return true;

    auto *callee = E->getDirectCallee();
    if (!callee || callee->isImplicit()) return true;

    auto calleeLoc = getLocation(callee->getLocation());
    // Allow calls to functions outside the repo (they won't have nodes, but the edge is still useful)
    std::string calleeFQN = getFQN(callee);
    std::string calleeSig = getSignature(callee);
    std::string calleeFile = calleeLoc.Valid ? calleeLoc.RelPath : "";

    std::string calleeLabel = "Function";
    if (llvm::isa<clang::CXXConstructorDecl>(callee))
        calleeLabel = "Constructor";

    std::string calleeID = makeNodeID(calleeLabel, calleeFile, calleeFQN, calleeSig);

    std::string edgeKey = callerID + " -CALLS-> " + calleeID;
    if (EmittedEdges.count(edgeKey)) return true;
    EmittedEdges.insert(edgeKey);

    Writer.emitEdge(callerID, calleeID, "CALLS");

    if (Verbose)
        llvm::errs() << "  CALLS: " << FunctionStack.back().FQN << " -> " << calleeFQN << "\n";

    return true;
}

bool ASTExtractor::VisitDeclRefExpr(clang::DeclRefExpr *E) {
    std::string callerID = getEnclosingFunctionID();
    if (callerID.empty()) return true;

    auto *D = E->getDecl();
    if (!D || D->isImplicit()) return true;

    // Only track references to global/file-scope variables
    auto *VD = llvm::dyn_cast<clang::VarDecl>(D);
    if (!VD || !VD->isFileVarDecl()) return true;

    auto refLoc = getLocation(VD->getLocation());
    if (!refLoc.Valid) return true;

    std::string refFQN = getFQN(VD);
    std::string refID = makeNodeID("Global", refLoc.RelPath, refFQN, "");

    std::string edgeKey = callerID + " -USES_GLOBAL-> " + refID;
    if (EmittedEdges.count(edgeKey)) return true;
    EmittedEdges.insert(edgeKey);

    Writer.emitEdge(callerID, refID, "USES_GLOBAL");

    return true;
}
