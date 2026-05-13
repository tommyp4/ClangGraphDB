#pragma once

#include "JSONLWriter.h"
#include "clang/Lex/PPCallbacks.h"
#include "clang/Basic/SourceManager.h"
#include <string>
#include <set>

class IncludeTracker : public clang::PPCallbacks {
public:
    IncludeTracker(clang::SourceManager &SM, JSONLWriter &Writer,
                   const std::string &RepoRoot, bool Verbose);

    void InclusionDirective(clang::SourceLocation HashLoc,
                            const clang::Token &IncludeTok,
                            llvm::StringRef FileName,
                            bool IsAngled,
                            clang::CharSourceRange FilenameRange,
                            clang::OptionalFileEntryRef File,
                            llvm::StringRef SearchPath,
                            llvm::StringRef RelativePath,
                            const clang::Module *SuggestedModule,
                            bool ModuleImported,
                            clang::SrcMgr::CharacteristicKind FileType) override;

private:
    std::string makeRelative(const std::string &absPath);
    bool isInsideRepo(const std::string &absPath);

    clang::SourceManager &SM;
    JSONLWriter &Writer;
    std::string RepoRoot;
    bool Verbose;
    std::set<std::string> EmittedEdges;
};
