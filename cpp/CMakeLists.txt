cmake_minimum_required(VERSION 3.20)
project(DNSEBridge LANGUAGES CXX)

add_library(DNSEBridge SHARED DNSEBridge.cpp)
target_compile_features(DNSEBridge PRIVATE cxx_std_17)
target_compile_definitions(DNSEBridge PRIVATE WIN32_LEAN_AND_MEAN NOMINMAX)
target_link_libraries(DNSEBridge PRIVATE ws2_32)

if(MSVC)
  target_compile_options(DNSEBridge PRIVATE /W4 /permissive-)
endif()
