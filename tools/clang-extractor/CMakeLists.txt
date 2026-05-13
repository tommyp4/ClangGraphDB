cmake_minimum_required(VERSION 3.20)
project(clang-extractor CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

find_package(LLVM REQUIRED CONFIG)
find_package(Clang REQUIRED CONFIG)

message(STATUS "Found LLVM ${LLVM_PACKAGE_VERSION}")
message(STATUS "Using LLVMConfig.cmake in: ${LLVM_DIR}")
message(STATUS "Using ClangConfig.cmake in: ${Clang_DIR}")

include_directories(${LLVM_INCLUDE_DIRS} ${CLANG_INCLUDE_DIRS})
link_directories(${LLVM_LIBRARY_DIRS})
add_definitions(${LLVM_DEFINITIONS})

add_executable(clang-extractor
    src/main.cpp
    src/ASTExtractor.cpp
    src/IncludeTracker.cpp
    src/JSONLWriter.cpp
)

# Use LLVM's cmake utilities to get the right libraries
llvm_map_components_to_libnames(llvm_libs
    Core
    Support
    Option
    FrontendOpenMP
)

target_link_libraries(clang-extractor PRIVATE
    clangTooling
    clangFrontend
    clangSerialization
    clangDriver
    clangParse
    clangSema
    clangAnalysis
    clangAST
    clangASTMatchers
    clangBasic
    clangEdit
    clangLex
    clangRewrite
    ${llvm_libs}
)

if(WIN32)
    target_link_libraries(clang-extractor PRIVATE version)
endif()
