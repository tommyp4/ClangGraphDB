#include "ASTExtractor.h"
#include "IncludeTracker.h"
#include "JSONLWriter.h"

#include "clang/Frontend/CompilerInstance.h"
#include "clang/Frontend/FrontendAction.h"
#include "clang/Tooling/CommonOptionsParser.h"
#include "clang/Tooling/Tooling.h"
#include "llvm/Support/CommandLine.h"
#include "llvm/Support/raw_ostream.h"

#include <atomic>
#include <memory>
#include <string>

static llvm::cl::OptionCategory ExtractorCategory("clang-extractor options");

static llvm::cl::opt<std::string> OutputDir(
    "o", llvm::cl::desc("Output directory for nodes.jsonl and edges.jsonl"),
    llvm::cl::value_desc("dir"), llvm::cl::Required,
    llvm::cl::cat(ExtractorCategory));

static llvm::cl::opt<std::string> RepoRoot(
    "root", llvm::cl::desc("Repository root for relative path computation"),
    llvm::cl::value_desc("dir"), llvm::cl::Required,
    llvm::cl::cat(ExtractorCategory));

static llvm::cl::opt<bool> Verbose(
    "verbose", llvm::cl::desc("Enable verbose output"),
    llvm::cl::cat(ExtractorCategory));

static std::atomic<int> FilesProcessed{0};
static std::atomic<int> FilesSucceeded{0};
static std::atomic<int> FilesFailed{0};

class ExtractorASTConsumer : public clang::ASTConsumer {
public:
    ExtractorASTConsumer(clang::CompilerInstance &CI, JSONLWriter &Writer,
                         const std::string &RepoRoot, bool Verbose)
        : Writer(Writer), Root(RepoRoot), Verb(Verbose), CI(CI) {}

    void HandleTranslationUnit(clang::ASTContext &Ctx) override {
        ASTExtractor extractor(Ctx, Writer, Root, Verb);
        extractor.TraverseDecl(Ctx.getTranslationUnitDecl());
    }

private:
    JSONLWriter &Writer;
    std::string Root;
    bool Verb;
    clang::CompilerInstance &CI;
};

class ExtractorAction : public clang::ASTFrontendAction {
public:
    ExtractorAction(JSONLWriter &Writer, const std::string &RepoRoot, bool Verbose)
        : Writer(Writer), Root(RepoRoot), Verb(Verbose) {}

    std::unique_ptr<clang::ASTConsumer>
    CreateASTConsumer(clang::CompilerInstance &CI, llvm::StringRef File) override {
        // Install include tracker as preprocessor callback
        CI.getPreprocessor().addPPCallbacks(
            std::make_unique<IncludeTracker>(
                CI.getSourceManager(), Writer, Root, Verb));

        return std::make_unique<ExtractorASTConsumer>(CI, Writer, Root, Verb);
    }

    void EndSourceFileAction() override {
        ++FilesProcessed;
        ++FilesSucceeded;
    }

private:
    JSONLWriter &Writer;
    std::string Root;
    bool Verb;
};

class ExtractorActionFactory : public clang::tooling::FrontendActionFactory {
public:
    ExtractorActionFactory(JSONLWriter &Writer, const std::string &RepoRoot, bool Verbose)
        : Writer(Writer), Root(RepoRoot), Verb(Verbose) {}

    std::unique_ptr<clang::FrontendAction> create() override {
        return std::make_unique<ExtractorAction>(Writer, Root, Verb);
    }

private:
    JSONLWriter &Writer;
    std::string Root;
    bool Verb;
};

int main(int argc, const char **argv) {
    auto ExpectedParser = clang::tooling::CommonOptionsParser::create(
        argc, argv, ExtractorCategory);
    if (!ExpectedParser) {
        llvm::errs() << ExpectedParser.takeError();
        return 1;
    }
    clang::tooling::CommonOptionsParser &OptionsParser = ExpectedParser.get();

    const auto &sources = OptionsParser.getSourcePathList();
    llvm::errs() << "clang-extractor: " << sources.size() << " files to process\n";
    llvm::errs() << "  output: " << OutputDir << "\n";
    llvm::errs() << "  root: " << RepoRoot << "\n";

    JSONLWriter writer(OutputDir);
    if (!writer.open()) {
        llvm::errs() << "ERROR: cannot open output files in " << OutputDir << "\n";
        return 1;
    }

    // Emit File nodes for all source files
    for (const auto &src : sources) {
        std::string normalized = src;
        std::replace(normalized.begin(), normalized.end(), '\\', '/');

        // Make relative
        std::string lower = normalized;
        std::string rootLower = RepoRoot.getValue();
        std::replace(rootLower.begin(), rootLower.end(), '\\', '/');
        if (!rootLower.empty() && rootLower.back() != '/')
            rootLower += '/';
        std::transform(lower.begin(), lower.end(), lower.begin(), ::tolower);
        std::transform(rootLower.begin(), rootLower.end(), rootLower.begin(), ::tolower);

        std::string relPath = normalized;
        if (lower.size() >= rootLower.size() &&
            lower.substr(0, rootLower.size()) == rootLower) {
            relPath = normalized.substr(rootLower.size());
        }

        std::string fileID = "File:" + relPath;
        writer.emitNode(fileID, "File", {
            {"name", relPath},
            {"file", relPath},
        });
    }

    clang::tooling::ClangTool tool(OptionsParser.getCompilations(),
                                    sources);

    // Process each file individually to handle errors gracefully
    ExtractorActionFactory factory(writer, RepoRoot, Verbose);
    int result = tool.run(&factory);

    // Files that failed are those where the action didn't complete
    int failed = static_cast<int>(sources.size()) - FilesSucceeded.load();

    writer.close();

    llvm::errs() << "\n=== clang-extractor summary ===\n";
    llvm::errs() << "  Files: " << sources.size() << " total, "
                 << FilesSucceeded.load() << " succeeded, "
                 << failed << " failed\n";
    llvm::errs() << "  Nodes: " << writer.nodeCount() << "\n";
    llvm::errs() << "  Edges: " << writer.edgeCount() << "\n";

    // Return 0 even if some files failed — partial results are still useful
    return 0;
}
