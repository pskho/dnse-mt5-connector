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
#include <unordered_map>
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

struct SymbolState {
    std::mutex tick_mutex;
    TickState tick;
    std::mutex history_mutex;
    std::vector<MqlRateLite> historical_rates;
    std::atomic<bool> history_ready{false};
    std::atomic<bool> running{false};
    std::atomic<bool> connected{false};
    std::mutex socket_mutex;
    SOCKET socket = INVALID_SOCKET;
    std::thread reader_thread;
    std::string endpoint;
    std::string symbol;
};

std::mutex g_states_mutex;
std::unordered_map<std::string, SymbolState*> g_states;
std::atomic<int> g_wsa_ref_count{0};

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

void close_socket_safe(SymbolState& state) {
    std::lock_guard<std::mutex> lock(state.socket_mutex);
    if (state.socket != INVALID_SOCKET) {
        shutdown(state.socket, SD_BOTH);
        closesocket(state.socket);
        state.socket = INVALID_SOCKET;
    }
    state.connected.store(false);
}

bool connect_socket(SymbolState& state) {
    const std::string& endpoint = state.endpoint;
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
        std::lock_guard<std::mutex> lock(state.socket_mutex);
        state.socket = s;
    }

    std::string subscribe = std::string("{\"type\":\"subscribe\",\"symbol\":\"") + state.symbol + "\"}\n";
    int sent = send(s, subscribe.c_str(), static_cast<int>(subscribe.size()), 0);
    if (sent != static_cast<int>(subscribe.size())) {
        log_msg("subscribe send failed for symbol " + state.symbol);
        closesocket(s);
        return false;
    }

    state.connected.store(true);
    log_msg("Successfully connected to " + endpoint + " for symbol " + state.symbol);
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

void reader_loop(SymbolState* state) {
    if (state == nullptr) return;
    log_msg("Reader thread started for " + state->symbol + " @ " + state->endpoint);
    std::string buffer;
    buffer.reserve(4096);

    while (state->running.load()) {
        if (!connect_socket(*state)) {
            // Wait with a loop so we can exit quickly if g_running becomes false
            for (int i = 0; i < 20 && state->running.load(); ++i) {
                Sleep(100);
            }
            continue;
        }

        char temp[2048];
        while (state->running.load() && state->connected.load()) {
            fd_set read_fds;
            FD_ZERO(&read_fds);
            
            SOCKET s;
            {
                std::lock_guard<std::mutex> lock(state->socket_mutex);
                s = state->socket;
            }
            if (s == INVALID_SOCKET) break;

            FD_SET(s, &read_fds);
            timeval timeout;
            timeout.tv_sec = 0;
            timeout.tv_usec = 500000; // 500ms
            
            int select_res = select(0, &read_fds, nullptr, nullptr, &timeout);
            if (!state->running.load()) break;

            if (select_res > 0) {
                int received = recv(s, temp, sizeof(temp), 0);
                if (received <= 0) {
                    log_msg("recv returned <= 0, disconnected for " + state->symbol);
                    close_socket_safe(*state);
                    break;
                }
                buffer.append(temp, temp + received);
                
                for (;;) {
                    auto newline = buffer.find('\n');
                    if (newline == std::string::npos) break;
                    std::string line = buffer.substr(0, newline);
                    buffer.erase(0, newline + 1);

                    if (contains_json_value(line, "type", "history_start")) {
                        std::lock_guard<std::mutex> lock(state->history_mutex);
                        state->historical_rates.clear();
                        state->history_ready.store(false);
                        continue;
                    }
                    if (contains_json_value(line, "type", "history_end")) {
                        state->history_ready.store(true);
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
                        std::lock_guard<std::mutex> lock(state->history_mutex);
                        state->historical_rates.push_back(c);
                    } else if (parse_tick_line(line, parsed)) {
                        std::lock_guard<std::mutex> lock(state->tick_mutex);
                        state->tick = parsed;
                    }
                }
                if (buffer.size() > 65536) {
                    log_msg("Buffer exceeded 64KB without newline, clearing to prevent OOM");
                    buffer.clear();
                }
            } else if (select_res == SOCKET_ERROR) {
                log_msg("select returned SOCKET_ERROR for " + state->symbol);
                close_socket_safe(*state);
                break;
            }
        }
        close_socket_safe(*state);
        log_msg("Disconnected from socket for " + state->symbol + ", retrying...");
        Sleep(500);
    }
    log_msg("Reader thread exiting for " + state->symbol);
}

} // namespace

SymbolState* get_state_for_symbol(const std::string& symbol) {
    std::lock_guard<std::mutex> lock(g_states_mutex);
    auto it = g_states.find(symbol);
    if (it == g_states.end()) return nullptr;
    return it->second;
}

extern "C" __declspec(dllexport) bool __stdcall ConnectBridge(const wchar_t* endpoint, const wchar_t* symbol) {
    std::string ep = wide_to_utf8(endpoint);
    std::string sym = wide_to_utf8(symbol);
    for (char& ch : sym) ch = static_cast<char>(std::toupper(static_cast<unsigned char>(ch)));
    log_msg("ConnectBridge called with endpoint: " + ep + ", symbol: " + sym);
    if (ep.empty() || sym.empty()) {
        log_msg("Endpoint is empty");
        return false;
    }

    SymbolState* state = get_state_for_symbol(sym);
    if (state != nullptr && state->running.load()) {
        log_msg("Already running for symbol " + sym);
        return true;
    }

    if (g_wsa_ref_count.fetch_add(1) == 0) {
        WSADATA data{};
        if (WSAStartup(MAKEWORD(2, 2), &data) != 0) {
            log_msg("WSAStartup failed");
            g_wsa_ref_count.fetch_sub(1);
            return false;
        }
    }

    if (state == nullptr) {
        state = new SymbolState();
        state->symbol = sym;
        state->endpoint = ep;
        std::lock_guard<std::mutex> lock(g_states_mutex);
        g_states[sym] = state;
    }

    state->endpoint = ep;
    state->history_ready.store(false);
    state->running.store(true);
    try {
        {
            std::lock_guard<std::mutex> lock(state->history_mutex);
            state->historical_rates.clear();
        }
        if (state->reader_thread.joinable()) {
            state->reader_thread.join();
        }
        state->reader_thread = std::thread(reader_loop, state);
    } catch (const std::exception& e) {
        log_msg("Exception creating thread: " + std::string(e.what()));
        state->running.store(false);
        if (g_wsa_ref_count.fetch_sub(1) == 1) {
            WSACleanup();
        }
        return false;
    }
    return true;
}

extern "C" __declspec(dllexport) bool __stdcall IsConnected(const wchar_t* symbol) {
    std::string sym = wide_to_utf8(symbol);
    for (char& ch : sym) ch = static_cast<char>(std::toupper(static_cast<unsigned char>(ch)));
    SymbolState* state = get_state_for_symbol(sym);
    return state != nullptr && state->connected.load();
}

extern "C" __declspec(dllexport) bool __stdcall IsHistoryReady(const wchar_t* symbol) {
    std::string sym = wide_to_utf8(symbol);
    for (char& ch : sym) ch = static_cast<char>(std::toupper(static_cast<unsigned char>(ch)));
    SymbolState* state = get_state_for_symbol(sym);
    return state != nullptr && state->history_ready.load();
}

extern "C" __declspec(dllexport) bool __stdcall GetLatestTick(
    const wchar_t* symbol,
    double* bid,
    double* ask,
    double* last,
    long long* volume,
    long long* timestamp_ms
) {
    if (bid == nullptr || ask == nullptr || last == nullptr || volume == nullptr || timestamp_ms == nullptr) {
        return false;
    }
    std::string sym = wide_to_utf8(symbol);
    for (char& ch : sym) ch = static_cast<char>(std::toupper(static_cast<unsigned char>(ch)));
    SymbolState* state = get_state_for_symbol(sym);
    if (state == nullptr) return false;
    std::lock_guard<std::mutex> lock(state->tick_mutex);
    if (!state->tick.valid) return false;
    *bid = state->tick.bid;
    *ask = state->tick.ask;
    *last = state->tick.last;
    *volume = state->tick.volume;
    *timestamp_ms = state->tick.timestamp_ms;
    return true;
}

extern "C" __declspec(dllexport) int __stdcall GetHistoricalRates(const wchar_t* symbol, MqlRateLite* buffer, int max_count) {
    if (buffer == nullptr || max_count <= 0) return 0;
    std::string sym = wide_to_utf8(symbol);
    for (char& ch : sym) ch = static_cast<char>(std::toupper(static_cast<unsigned char>(ch)));
    SymbolState* state = get_state_for_symbol(sym);
    if (state == nullptr) return 0;
    std::lock_guard<std::mutex> lock(state->history_mutex);
    int count = static_cast<int>(state->historical_rates.size());
    if (count == 0) return 0;
    int copy_count = (count > max_count) ? max_count : count;
    std::memcpy(buffer, state->historical_rates.data(), copy_count * sizeof(MqlRateLite));
    return copy_count;
}

extern "C" __declspec(dllexport) int __stdcall GetHistoricalRatesCount(const wchar_t* symbol) {
    std::string sym = wide_to_utf8(symbol);
    for (char& ch : sym) ch = static_cast<char>(std::toupper(static_cast<unsigned char>(ch)));
    SymbolState* state = get_state_for_symbol(sym);
    if (state == nullptr) return 0;
    std::lock_guard<std::mutex> lock(state->history_mutex);
    return static_cast<int>(state->historical_rates.size());
}

extern "C" __declspec(dllexport) int __stdcall GetHistoricalRatesRange(const wchar_t* symbol, int offset, MqlRateLite* buffer, int max_count) {
    if (buffer == nullptr || max_count <= 0) return 0;
    std::string sym = wide_to_utf8(symbol);
    for (char& ch : sym) ch = static_cast<char>(std::toupper(static_cast<unsigned char>(ch)));
    SymbolState* state = get_state_for_symbol(sym);
    if (state == nullptr) return 0;
    std::lock_guard<std::mutex> lock(state->history_mutex);
    int count = static_cast<int>(state->historical_rates.size());
    if (count == 0) return 0;
    if (offset < 0 || offset >= count) return 0;
    int available = count - offset;
    int copy_count = (available > max_count) ? max_count : available;
    std::memcpy(buffer, state->historical_rates.data() + offset, copy_count * sizeof(MqlRateLite));
    return copy_count;
}

extern "C" __declspec(dllexport) void __stdcall ClearHistoricalRates(const wchar_t* symbol) {
    std::string sym = wide_to_utf8(symbol);
    for (char& ch : sym) ch = static_cast<char>(std::toupper(static_cast<unsigned char>(ch)));
    SymbolState* state = get_state_for_symbol(sym);
    if (state == nullptr) return;
    std::lock_guard<std::mutex> lock(state->history_mutex);
    state->historical_rates.clear();
    state->history_ready.store(false);
}

extern "C" __declspec(dllexport) void __stdcall DisconnectBridge(const wchar_t* symbol) {
    std::string sym = wide_to_utf8(symbol);
    for (char& ch : sym) ch = static_cast<char>(std::toupper(static_cast<unsigned char>(ch)));
    SymbolState* state = get_state_for_symbol(sym);
    if (state == nullptr) return;
    log_msg("DisconnectBridge called for " + sym);
    state->running.store(false);
    close_socket_safe(*state);

    if (state->reader_thread.joinable()) {
        state->reader_thread.join();
    }

    {
        std::lock_guard<std::mutex> lock(state->tick_mutex);
        state->tick = TickState{};
    }
    {
        std::lock_guard<std::mutex> lock(state->history_mutex);
        state->historical_rates.clear();
    }
    state->history_ready.store(false);
    {
        std::lock_guard<std::mutex> lock(g_states_mutex);
        g_states.erase(sym);
    }
    delete state;
    if (g_wsa_ref_count.fetch_sub(1) == 1) {
        WSACleanup();
    }
    log_msg("Disconnected completely for " + sym);
}

BOOL APIENTRY DllMain(HMODULE, DWORD reason, LPVOID) {
    if (reason == DLL_PROCESS_DETACH) {
        std::vector<SymbolState*> states;
        {
            std::lock_guard<std::mutex> lock(g_states_mutex);
            for (auto& entry : g_states) {
                states.push_back(entry.second);
            }
            g_states.clear();
        }
        for (SymbolState* state : states) {
            state->running.store(false);
            close_socket_safe(*state);
            if (state->reader_thread.joinable()) {
                state->reader_thread.join();
            }
            delete state;
        }
        if (g_wsa_ref_count.load() > 0) {
            WSACleanup();
        }
    }
    return TRUE;
}
