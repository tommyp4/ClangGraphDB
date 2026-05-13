#include "IncludeTracker.h"
#include "llvm/Support/Path.h"
#include "llvm/Support/raw_ostream.h"
#include <algorithm>

IncludeTracker::IncludeTracker(clang::SourceManager &SM, JSONLWriter &Writer,
                               const std::string &RepoRoot, bool Verbose)
    : SM(SM), Writer(Writer), RepoRoot(RepoRoot), Verbose(Verbose) {
    // Normalize repo root to forward slashes with trailing separator
    std::replace(this->RepoRoot.begin(), this->RepoRoot.end(), '\\', '/');
    if (!this->RepoRoot.empty() && this->RepoRoot.back() != '/')
        this->RepoRoot += '/';
    // Lowercase for case-insensitive comparison on Windows
    std::transform(this->RepoRoot.begin(), this->RepoRoot.end(),
                   this->RepoRoot.begin(), ::tolower);
}

void IncludeTracker::InclusionDirective(
    clang::SourceLocation HashLoc, const clang::Token &IncludeTok,
    llvm::StringRef FileName, bool IsAngled,
    clang::CharSourceRange FilenameRange, clang::OptionalFileEntryRef File,
    llvm::StringRef SearchPath, llvm::StringRef RelativePath,
    const clang::Module *SuggestedModule, bool ModuleImported,
    clang::SrcMgr::CharacteristicKind FileType) {

    if (!File)
        return;

    std::string includedPath = File->getFileEntry().tryGetRealPathName().str();
    if (includedPath.empty())
        includedPath = File->getFileEntry().getName().str();

    if (!isInsideRepo(includedPath))
        return;

    clang::SourceLocation expansionLoc = SM.getExpansionLoc(HashLoc);
    if (expansionLoc.isInvalid())
        return;

    auto includerEntry = SM.getFileEntryRefForID(SM.getFileID(expansionLoc));
    if (!includerEntry)
        return;

    std::string includerPath = includerEntry->getFileEntry().tryGetRealPathName().str();
    if (includerPath.empty())
        includerPath = includerEntry->getFileEntry().getName().str();

    if (!isInsideRepo(includerPath))
        return;

    std::string relIncluder = makeRelative(includerPath);
    std::string relIncluded = makeRelative(includedPath);

    std::string edgeKey = relIncluder + " -> " + relIncluded;
    if (EmittedEdges.count(edgeKey))
        return;
    EmittedEdges.insert(edgeKey);

    std::string sourceId = "File:" + relIncluder;
    std::string targetId = "File:" + relIncluded;

    Writer.emitEdge(sourceId, targetId, "INCLUDES");

    if (Verbose)
        llvm::errs() << "  INCLUDES: " << relIncluder << " -> " << relIncluded << "\n";
}

std::string IncludeTracker::makeRelative(const std::string &absPath) {
    std::string normalized = absPath;
    std::replace(normalized.begin(), normalized.end(), '\\', '/');

    std::string lower = normalized;
    std::transform(lower.begin(), lower.end(), lower.begin(), ::tolower);

    if (lower.substr(0, RepoRoot.size()) == RepoRoot)
        return normalized.substr(RepoRoot.size());

    return normalized;
}

bool IncludeTracker::isInsideRepo(const std::string &absPath) {
    std::string normalized = absPath;
    std::replace(normalized.begin(), normalized.end(), '\\', '/');
    std::transform(normalized.begin(), normalized.end(), normalized.begin(), ::tolower);
    return normalized.substr(0, RepoRoot.size()) == RepoRoot;
}
