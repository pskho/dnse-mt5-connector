#define WIN32_LEAN_AND_MEAN
#include <winsock2.h>
#include <ws2tcpip.h>
#include <windows.h>

#include <atomic>
#include <cctype>
#include <cstring>
#include <cstdlib>
#include <mutex>
#include <string>
#include <thread>
#include <vector>
#include <fstream>
#include <ctime>

#pragma comment(lib, "Ws2_32.lib")

namespace {

void log_msg(const std::string& msg) {
    static std::mutex mtx;
    std::lock_guard<std::mutex> lock(mtx);
    std::ofstream ofs("dnse_bridge.log", std::ios_base::app);
    if (ofs.is_open()) {
        char timebuf[64];
        time_t now = time(nullptr);
        tm tinfo;
        localtime_s(&tinfo, &now);
        strftime(timebuf, sizeof(timebuf), "%Y-%m-%d %H:%M:%S", &tinfo);
        ofs << "[" << timebuf << "] " << msg << std::endl;
    }
}

struct TickState {
    double bid = 0.0;
    double ask = 0.0;
    double last = 0.0;
    long long volume = 0;
    long long timestamp_ms = 0;
    bool valid = false;
};

#pragma pack(push, 1)
struct MqlRateLite {
    long long time;
    double open;
    double high;
    double low;
    double close;
    long long tick_volume;
};
#pragma pack(pop)

std::mutex g_tick_mutex;
TickState g_tick;

std::mutex g_history_mutex;
std::vector<MqlRateLite> g_historical_rates;
std::atomic<bool> g_history_ready{false};

std::atomic<bool> g_running{false};
std::atomic<bool> g_connected{false};

std::mutex g_socket_mutex;
SOCKET g_socket = INVALID_SOCKET;

std::thread g_reader_thread;

std::string wide_to_utf8(const wchar_t* value) {
    if (value == nullptr) return {};
    int size = WideCharToMultiByte(CP_UTF8, 0, value, -1, nullptr, 0, nullptr, nullptr);
    if (size <= 1) return {};
    std::vector<char> out(size);
    WideCharToMultiByte(CP_UTF8, 0, value, -1, out.data(), size, nullptr, nullptr);
    return std::string(out.data());
}

bool split_endpoint(const std::string& endpoint, std::string& host, std::string& port) {
    std::string value = endpoint;
    const std::string prefix = "tcp://";
    if (value.rfind(prefix, 0) == 0) value = value.substr(prefix.size());
    auto pos = value.rfind(':');
    if (pos == std::string::npos || pos == 0 || pos + 1 >= value.size()) return false;
    host = value.substr(0, pos);
    port = value.substr(pos + 1);
    return true;
}

void close_socket_safe() {
    std::lock_guard<std::mutex> lock(g_socket_mutex);
    if (g_socket != INVALID_SOCKET) {
        shutdown(g_socket, SD_BOTH);
        closesocket(g_socket);
        g_socket = INVALID_SOCKET;
    }
    g_connected.store(false);
}

bool connect_socket(const std::string& endpoint) {
    std::string host, port;
    if (!split_endpoint(endpoint, host, port)) {
        log_msg("Invalid endpoint format: " + endpoint);
        return false;
    }

    addrinfo hints{};
    hints.ai_family = AF_UNSPEC;
    hints.ai_socktype = SOCK_STREAM;
    hints.ai_protocol = IPPROTO_TCP;

    addrinfo* result = nullptr;
    if (getaddrinfo(host.c_str(), port.c_str(), &hints, &result) != 0) {
        log_msg("getaddrinfo failed for " + host + ":" + port);
        return false;
    }

    SOCKET s = INVALID_SOCKET;
    for (addrinfo* ptr = result; ptr != nullptr; ptr = ptr->ai_next) {
        s = socket(ptr->ai_family, ptr->ai_socktype, ptr->ai_protocol);
        if (s == INVALID_SOCKET) continue;
        if (connect(s, ptr->ai_addr, static_cast<int>(ptr->ai_addrlen)) == 0) {
            break;
        }
        closesocket(s);
        s = INVALID_SOCKET;
    }
    freeaddrinfo(result);
    
    if (s == INVALID_SOCKET) {
        log_msg("connect failed to " + endpoint);
        return false;
    }

    // Set non-blocking mode for select()
    u_long non_blocking = 1;
    ioctlsocket(s, FIONBIO, &non_blocking);
    
    {
        std::lock_guard<std::mutex> lock(g_socket_mutex);
        g_socket = s;
    }
    
    g_connected.store(true);
    log_msg("Successfully connected to " + endpoint);
    return true;
}

double parse_double(const std::string& json, const char* key) {
    std::string needle = std::string("\"") + key + "\"";
    auto pos = json.find(needle);
    if (pos == std::string::npos) return 0.0;
    pos = json.find(':', pos + needle.size());
    if (pos == std::string::npos) return 0.0;
    ++pos;
    while (pos < json.size() && std::isspace(static_cast<unsigned char>(json[pos]))) ++pos;
    return std::strtod(json.c_str() + pos, nullptr);
}

long long parse_int64(const std::string& json, const char* key) {
    std::string needle = std::string("\"") + key + "\"";
    auto pos = json.find(needle);
    if (pos == std::string::npos) return 0;
    pos = json.find(':', pos + needle.size());
    if (pos == std::string::npos) return 0;
    ++pos;
    while (pos < json.size() && std::isspace(static_cast<unsigned char>(json[pos]))) ++pos;
    return _strtoi64(json.c_str() + pos, nullptr, 10);
}

bool parse_tick_line(const std::string& line, TickState& tick) {
    TickState parsed;
    parsed.bid = parse_double(line, "bid");
    parsed.ask = parse_double(line, "ask");
    parsed.last = parse_double(line, "last");
    parsed.volume = parse_int64(line, "volume");
    parsed.timestamp_ms = parse_int64(line, "timestamp_ms");
    parsed.valid = parsed.timestamp_ms > 0 && (parsed.bid > 0.0 || parsed.ask > 0.0 || parsed.last > 0.0);
    if (!parsed.valid) return false;
    tick = parsed;
    return true;
}

bool contains_json_value(const std::string& json, const char* key, const char* expected) {
    std::string needle = std::string("\"") + key + "\"";
    auto pos = json.find(needle);
    if (pos == std::string::npos) return false;
    pos = json.find(':', pos + needle.size());
    if (pos == std::string::npos) return false;
    ++pos;
    while (pos < json.size() && std::isspace(static_cast<unsigned char>(json[pos]))) ++pos;
    if (pos >= json.size() || json[pos] != '"') return false;
    ++pos;
    return json.compare(pos, std::strlen(expected), expected) == 0;
}

void reader_loop(std::string endpoint) {
    log_msg("Reader thread started for " + endpoint);
    std::string buffer;
    buffer.reserve(4096);

    while (g_running.load()) {
        if (!connect_socket(endpoint)) {
            // Wait with a loop so we can exit quickly if g_running becomes false
            for (int i = 0; i < 20 && g_running.load(); ++i) {
                Sleep(100);
            }
            continue;
        }

        char temp[2048];
        while (g_running.load() && g_connected.load()) {
            fd_set read_fds;
            FD_ZERO(&read_fds);
            
            SOCKET s;
            {
                std::lock_guard<std::mutex> lock(g_socket_mutex);
                s = g_socket;
            }
            if (s == INVALID_SOCKET) break;

            FD_SET(s, &read_fds);
            timeval timeout;
            timeout.tv_sec = 0;
            timeout.tv_usec = 500000; // 500ms
            
            int select_res = select(0, &read_fds, nullptr, nullptr, &timeout);
            if (!g_running.load()) break;

            if (select_res > 0) {
                int received = recv(s, temp, sizeof(temp), 0);
                if (received <= 0) {
                    log_msg("recv returned <= 0, disconnected");
                    close_socket_safe();
                    break;
                }
                buffer.append(temp, temp + received);
                
                for (;;) {
                    auto newline = buffer.find('\n');
                    if (newline == std::string::npos) break;
                    std::string line = buffer.substr(0, newline);
                    buffer.erase(0, newline + 1);

                    if (contains_json_value(line, "type", "history_start")) {
                        std::lock_guard<std::mutex> lock(g_history_mutex);
                        g_historical_rates.clear();
                        g_history_ready.store(false);
                        continue;
                    }
                    if (contains_json_value(line, "type", "history_end")) {
                        g_history_ready.store(true);
                        continue;
                    }

                    TickState parsed;
                    long long is_hist = parse_int64(line, "is_history");
                    if (is_hist == 1) {
                        MqlRateLite c;
                        c.time = parse_int64(line, "time");
                        c.open = parse_double(line, "open");
                        c.high = parse_double(line, "high");
                        c.low = parse_double(line, "low");
                        c.close = parse_double(line, "close");
                        c.tick_volume = parse_int64(line, "tick_volume");
                        std::lock_guard<std::mutex> lock(g_history_mutex);
                        g_historical_rates.push_back(c);
                    } else if (parse_tick_line(line, parsed)) {
                        std::lock_guard<std::mutex> lock(g_tick_mutex);
                        g_tick = parsed;
                    }
                }
                if (buffer.size() > 65536) {
                    log_msg("Buffer exceeded 64KB without newline, clearing to prevent OOM");
                    buffer.clear();
                }
            } else if (select_res == SOCKET_ERROR) {
                log_msg("select returned SOCKET_ERROR");
                close_socket_safe();
                break;
            }
        }
        close_socket_safe();
        log_msg("Disconnected from socket, retrying...");
        Sleep(500);
    }
    log_msg("Reader thread exiting");
}

} // namespace

extern "C" __declspec(dllexport) bool __stdcall ConnectBridge(const wchar_t* endpoint) {
    std::string ep = wide_to_utf8(endpoint);
    log_msg("ConnectBridge called with endpoint: " + ep);
    if (ep.empty()) {
        log_msg("Endpoint is empty");
        return false;
    }
    if (g_running.load()) {
        log_msg("Already running");
        return true;
    }

    WSADATA data{};
    if (WSAStartup(MAKEWORD(2, 2), &data) != 0) {
        log_msg("WSAStartup failed");
        return false;
    }

    g_running.store(true);
    g_history_ready.store(false);
    try {
        {
            std::lock_guard<std::mutex> lock(g_history_mutex);
            g_historical_rates.clear();
        }
        if (g_reader_thread.joinable()) {
            g_reader_thread.join();
        }
        g_reader_thread = std::thread(reader_loop, ep);
    } catch (const std::exception& e) {
        log_msg("Exception creating thread: " + std::string(e.what()));
        g_running.store(false);
        WSACleanup();
        return false;
    }
    return true;
}

extern "C" __declspec(dllexport) bool __stdcall IsConnected() {
    return g_connected.load();
}

extern "C" __declspec(dllexport) bool __stdcall IsHistoryReady() {
    return g_history_ready.load();
}

extern "C" __declspec(dllexport) bool __stdcall GetLatestTick(
    double* bid,
    double* ask,
    double* last,
    long long* volume,
    long long* timestamp_ms
) {
    if (bid == nullptr || ask == nullptr || last == nullptr || volume == nullptr || timestamp_ms == nullptr) {
        return false;
    }
    std::lock_guard<std::mutex> lock(g_tick_mutex);
    if (!g_tick.valid) return false;
    *bid = g_tick.bid;
    *ask = g_tick.ask;
    *last = g_tick.last;
    *volume = g_tick.volume;
    *timestamp_ms = g_tick.timestamp_ms;
    return true;
}

extern "C" __declspec(dllexport) int __stdcall GetHistoricalRates(MqlRateLite* buffer, int max_count) {
    if (buffer == nullptr || max_count <= 0) return 0;
    std::lock_guard<std::mutex> lock(g_history_mutex);
    int count = static_cast<int>(g_historical_rates.size());
    if (count == 0) return 0;
    int copy_count = (count > max_count) ? max_count : count;
    std::memcpy(buffer, g_historical_rates.data(), copy_count * sizeof(MqlRateLite));
    return copy_count;
}

extern "C" __declspec(dllexport) int __stdcall GetHistoricalRatesCount() {
    std::lock_guard<std::mutex> lock(g_history_mutex);
    return static_cast<int>(g_historical_rates.size());
}

extern "C" __declspec(dllexport) int __stdcall GetHistoricalRatesRange(int offset, MqlRateLite* buffer, int max_count) {
    if (buffer == nullptr || max_count <= 0) return 0;
    std::lock_guard<std::mutex> lock(g_history_mutex);
    int count = static_cast<int>(g_historical_rates.size());
    if (count == 0) return 0;
    if (offset < 0 || offset >= count) return 0;
    int available = count - offset;
    int copy_count = (available > max_count) ? max_count : available;
    std::memcpy(buffer, g_historical_rates.data() + offset, copy_count * sizeof(MqlRateLite));
    return copy_count;
}

extern "C" __declspec(dllexport) void __stdcall ClearHistoricalRates() {
    std::lock_guard<std::mutex> lock(g_history_mutex);
    g_historical_rates.clear();
    g_history_ready.store(false);
}

extern "C" __declspec(dllexport) void __stdcall DisconnectBridge() {
    log_msg("DisconnectBridge called");
    g_running.store(false);
    close_socket_safe();
    
    if (g_reader_thread.joinable()) {
        g_reader_thread.join();
    }

    {
        std::lock_guard<std::mutex> lock(g_tick_mutex);
        g_tick = TickState{};
    }
    {
        std::lock_guard<std::mutex> lock(g_history_mutex);
        g_historical_rates.clear();
    }
    g_history_ready.store(false);
    WSACleanup();
    log_msg("Disconnected completely");
}

BOOL APIENTRY DllMain(HMODULE, DWORD reason, LPVOID) {
    if (reason == DLL_PROCESS_DETACH) {
        g_running.store(false);
        close_socket_safe();
        if (g_reader_thread.joinable()) {
            g_reader_thread.join();
        }
        WSACleanup();
    }
    return TRUE;
}
