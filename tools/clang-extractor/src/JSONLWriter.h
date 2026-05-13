#pragma once

#include <fstream>
#include <mutex>
#include <string>
#include <vector>
#include <utility>

class JSONLWriter {
public:
    JSONLWriter(const std::string &outputDir);
    ~JSONLWriter();

    bool open();
    void close();

    void emitNode(const std::string &id, const std::string &type,
                  const std::vector<std::pair<std::string, std::string>> &props);

    void emitEdge(const std::string &source, const std::string &target,
                  const std::string &type);

    size_t nodeCount() const { return NodeCount; }
    size_t edgeCount() const { return EdgeCount; }

private:
    static std::string escapeJSON(const std::string &s);

    std::string OutputDir;
    std::ofstream NodesFile;
    std::ofstream EdgesFile;
    std::mutex NodesMutex;
    std::mutex EdgesMutex;
    size_t NodeCount = 0;
    size_t EdgeCount = 0;
};
