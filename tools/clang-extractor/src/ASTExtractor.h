#pragma once

#include "JSONLWriter.h"
#include "clang/AST/RecursiveASTVisitor.h"
#include "clang/AST/ASTContext.h"
#include "clang/Basic/SourceManager.h"
#include <string>
#include <set>

class ASTExtractor : public clang::RecursiveASTVisitor<ASTExtractor> {
public:
    ASTExtractor(clang::ASTContext &Ctx, JSONLWriter &Writer,
                 const std::string &RepoRoot, bool Verbose);

    bool TraverseFunctionDecl(clang::FunctionDecl *D);
    bool TraverseCXXMethodDecl(clang::CXXMethodDecl *D);
    bool TraverseCXXConstructorDecl(clang::CXXConstructorDecl *D);
    bool TraverseCXXDestructorDecl(clang::CXXDestructorDecl *D);
    bool VisitFunctionDecl(clang::FunctionDecl *D);
    bool VisitCXXRecordDecl(clang::CXXRecordDecl *D);
    bool VisitVarDecl(clang::VarDecl *D);
    bool VisitFieldDecl(clang::FieldDecl *D);
    bool VisitCallExpr(clang::CallExpr *E);
    bool VisitDeclRefExpr(clang::DeclRefExpr *E);

private:
    struct LocationInfo {
        std::string RelPath;
        unsigned StartLine;
        unsigned EndLine;
        bool Valid;
    };

    LocationInfo getLocation(clang::SourceLocation loc);
    LocationInfo getRange(clang::SourceRange range);
    std::string makeRelative(const std::string &absPath);
    bool isInsideRepo(const std::string &absPath);
    std::string getFQN(const clang::NamedDecl *D);
    std::string getSignature(const clang::FunctionDecl *D);
    std::string makeNodeID(const std::string &label, const std::string &file,
                           const std::string &fqn, const std::string &sig);
    std::string getEnclosingFunctionID();

    clang::ASTContext &Ctx;
    clang::SourceManager &SM;
    JSONLWriter &Writer;
    std::string RepoRoot;
    bool Verbose;
    std::set<std::string> EmittedNodes;
    std::set<std::string> EmittedEdges;

    // Track current function context for CALLS/USES edges
    struct FunctionContext {
        std::string NodeID;
        std::string FQN;
    };
    std::vector<FunctionContext> FunctionStack;
};
