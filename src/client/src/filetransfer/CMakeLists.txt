# File Transfer Module CMakeLists.txt

# Find required Qt components
find_package(Qt6 REQUIRED COMPONENTS Core Widgets Network WebSockets)

# Define the file transfer module sources
set(FILETRANSFER_SOURCES
    FileTransferManager.cpp
    FileTransferSession.cpp
    FileTransferWorker.cpp
    transfer_dialog.cpp
    progress_widget.cpp
    ApprovalDialog.cpp
)

# Define the file transfer module headers
set(FILETRANSFER_HEADERS
    FileTransferManager.h
    FileTransferSession.h
    FileTransferWorker.h
    transfer_dialog.h
    progress_widget.h
    ApprovalDialog.h
)

# Create the file transfer library
add_library(filetransfer STATIC
    ${FILETRANSFER_SOURCES}
    ${FILETRANSFER_HEADERS}
)

# Set target properties
set_target_properties(filetransfer PROPERTIES
    CXX_STANDARD 17
    CXX_STANDARD_REQUIRED ON
    AUTOMOC ON
    AUTORCC ON
    AUTOUIC ON
)

# Link Qt libraries
target_link_libraries(filetransfer
    Qt6::Core
    Qt6::Widgets
    Qt6::Network
    Qt6::WebSockets
)

# Include directories
target_include_directories(filetransfer
    PUBLIC
        ${CMAKE_CURRENT_SOURCE_DIR}
    PRIVATE
        ${CMAKE_CURRENT_SOURCE_DIR}/../
)

# Compiler definitions
target_compile_definitions(filetransfer
    PRIVATE
        QT_NO_KEYWORDS
        QT_USE_QSTRINGBUILDER
)

# Enable warnings
if(MSVC)
    target_compile_options(filetransfer PRIVATE /W4)
else()
    target_compile_options(filetransfer PRIVATE -Wall -Wextra -Wpedantic)
endif()

# Install targets
install(TARGETS filetransfer
    LIBRARY DESTINATION lib
    ARCHIVE DESTINATION lib
    RUNTIME DESTINATION bin
)

# Install headers
install(FILES ${FILETRANSFER_HEADERS}
    DESTINATION include/filetransfer
)