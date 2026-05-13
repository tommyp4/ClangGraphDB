#include "JSONLWriter.h"
#include "llvm/Support/FileSystem.h"
#include "llvm/Support/Path.h"
#include <sstream>

JSONLWriter::JSONLWriter(const std::string &outputDir) : OutputDir(outputDir) {}

JSONLWriter::~JSONLWriter() { close(); }

bool JSONLWriter::open() {
    llvm::sys::fs::create_directories(OutputDir);

    std::string nodesPath = OutputDir + "/nodes.jsonl";
    std::string edgesPath = OutputDir + "/edges.jsonl";

    NodesFile.open(nodesPath, std::ios::out | std::ios::trunc);
    EdgesFile.open(edgesPath, std::ios::out | std::ios::trunc);

    return NodesFile.is_open() && EdgesFile.is_open();
}

void JSONLWriter::close() {
    if (NodesFile.is_open()) NodesFile.close();
    if (EdgesFile.is_open()) EdgesFile.close();
}

void JSONLWriter::emitNode(const std::string &id, const std::string &type,
                           const std::vector<std::pair<std::string, std::string>> &props) {
    std::ostringstream ss;
    ss << "{\"id\":\"" << escapeJSON(id) << "\",\"type\":\"" << escapeJSON(type) << "\"";
    for (const auto &p : props) {
        ss << ",\"" << escapeJSON(p.first) << "\":\"" << escapeJSON(p.second) << "\"";
    }
    ss << "}\n";

    std::lock_guard<std::mutex> lock(NodesMutex);
    NodesFile << ss.str();
    ++NodeCount;
}

void JSONLWriter::emitEdge(const std::string &source, const std::string &target,
                           const std::string &type) {
    std::ostringstream ss;
    ss << "{\"source\":\"" << escapeJSON(source)
       << "\",\"target\":\"" << escapeJSON(target)
       << "\",\"type\":\"" << escapeJSON(type) << "\"}\n";

    std::lock_guard<std::mutex> lock(EdgesMutex);
    EdgesFile << ss.str();
    ++EdgeCount;
}

std::string JSONLWriter::escapeJSON(const std::string &s) {
    std::string result;
    result.reserve(s.size());
    for (char c : s) {
        switch (c) {
        case '"':  result += "\\\""; break;
        case '\\': result += "\\\\"; break;
        case '\n': result += "\\n"; break;
        case '\r': result += "\\r"; break;
        case '\t': result += "\\t"; break;
        default:   result += c; break;
        }
    }
    return result;
}
