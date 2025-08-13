cmake_minimum_required(VERSION 3.16)

project(FileTransferManagerTest)

# Find required Qt components
find_package(Qt6 REQUIRED COMPONENTS Core Widgets Network WebSockets Test)

# Set C++ standard
set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

# Enable Qt's automatic MOC, UIC, and RCC processing
set(CMAKE_AUTOMOC ON)
set(CMAKE_AUTORCC ON)
set(CMAKE_AUTOUIC ON)

# Include directories
include_directories(
    ${CMAKE_CURRENT_SOURCE_DIR}
    ${CMAKE_CURRENT_SOURCE_DIR}/../../../src/client/src
    ${CMAKE_CURRENT_SOURCE_DIR}/../../../src/client/src/filetransfer
)

# Source files for the test
set(TEST_SOURCES
    FileTransferManagerTest.cpp
    ../../../src/client/src/filetransfer/FileTransferManager.cpp
    ../../../src/client/src/filetransfer/ApprovalDialog.cpp
    # Add other source files as needed
)

# Create the test executable
add_executable(FileTransferManagerTest ${TEST_SOURCES})

# Link Qt libraries
target_link_libraries(FileTransferManagerTest
    Qt6::Core
    Qt6::Widgets
    Qt6::Network
    Qt6::WebSockets
    Qt6::Test
)

# Add compiler flags for testing
target_compile_definitions(FileTransferManagerTest PRIVATE
    QT_TESTCASE_BUILDDIR="${CMAKE_CURRENT_BINARY_DIR}"
)

# Enable testing
enable_testing()

# Add the test to CTest
add_test(NAME FileTransferManagerTest COMMAND FileTransferManagerTest)

# Set test properties
set_tests_properties(FileTransferManagerTest PROPERTIES
    TIMEOUT 30
    ENVIRONMENT "QT_QPA_PLATFORM=offscreen"
)

# Optional: Add custom target to run tests
add_custom_target(run_file_transfer_tests
    COMMAND ${CMAKE_CTEST_COMMAND} --output-on-failure
    DEPENDS FileTransferManagerTest
    COMMENT "Running File Transfer Manager tests"
)