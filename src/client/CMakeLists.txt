cmake_minimum_required(VERSION 3.16)

project(OnlideskClient VERSION 1.0.0 LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

# Find Qt6 components
find_package(Qt6 REQUIRED COMPONENTS Core Widgets Network WebSockets)

# Enable Qt MOC, UIC, and RCC
set(CMAKE_AUTOMOC ON)
set(CMAKE_AUTOUIC ON)
set(CMAKE_AUTORCC ON)

# Include directories
include_directories(include)

# Source files
set(SOURCES
    src/main.cpp
    src/client.cpp
    src/session_manager.cpp
    src/screen_capture.cpp
    src/input_handler.cpp
    src/file_transfer.cpp
    src/network_manager.cpp
)

# Header files
set(HEADERS
    include/client.h
    include/session_manager.h
    include/screen_capture.h
    include/input_handler.h
    include/file_transfer.h
    include/network_manager.h
)

# UI files
set(UI_FILES
    # Add .ui files here when created
)

# Resource files
set(RESOURCES
    resources/client.qrc
)

# Create executable
add_executable(OnlideskClient
    ${SOURCES}
    ${HEADERS}
    ${UI_FILES}
    ${RESOURCES}
)

# Link Qt libraries
target_link_libraries(OnlideskClient
    Qt6::Core
    Qt6::Widgets
    Qt6::Network
    Qt6::WebSockets
)

# Platform-specific configurations
if(WIN32)
    # Windows specific settings
    set_target_properties(OnlideskClient PROPERTIES
        WIN32_EXECUTABLE TRUE
    )
elseif(APPLE)
    # macOS specific settings
    set_target_properties(OnlideskClient PROPERTIES
        MACOSX_BUNDLE TRUE
    )
endif()

# Install target
install(TARGETS OnlideskClient
    BUNDLE DESTINATION .
    RUNTIME DESTINATION bin
)