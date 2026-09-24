/// Application settings mirror (backend/internal/settings JSON).
class AppSettings {
  final bool autoConnect;
  final bool autoMode;
  final String routingMode;
  final String dnsMode;
  final bool tunEnabled;
  final bool killSwitch;
  final int mixedPort;
  final int testTimeoutMs;
  final bool updateOnStart;
  final String theme;
  final int urlTestIntervalMin;
  final String connectionTestUrl;
  final int clashApiPort;
  final String corePath;
  final String helperPath;
  final int mtu;
  final int testConcurrency;
  final int healthIntervalSec;
  final String updateChannel;
  final bool updateAutoCheck;
  final String splitMode;
  final List<String> splitApps;

  const AppSettings({
    this.autoConnect = false,
    this.autoMode = true,
    this.routingMode = 'global',
    this.dnsMode = 'proxy-aware',
    this.tunEnabled = true,
    this.killSwitch = false,
    this.mixedPort = 10808,
    this.testTimeoutMs = 5000,
    this.updateOnStart = true,
    this.theme = 'dark',
    this.urlTestIntervalMin = 10,
    this.connectionTestUrl = 'http://connectivitycheck.gstatic.com/generate_204',
    this.clashApiPort = 16756,
    this.corePath = '',
    this.helperPath = '',
    this.mtu = 9000,
    this.testConcurrency = 5,
    this.healthIntervalSec = 30,
    this.updateChannel = 'stable',
    this.updateAutoCheck = true,
    this.splitMode = 'off',
    this.splitApps = const [],
  });

  AppSettings copyWith({
    bool? autoConnect,
    bool? autoMode,
    String? routingMode,
    String? dnsMode,
    bool? tunEnabled,
    bool? killSwitch,
    int? mixedPort,
    int? testTimeoutMs,
    bool? updateOnStart,
    String? theme,
    int? urlTestIntervalMin,
    String? connectionTestUrl,
    int? clashApiPort,
    String? corePath,
    String? helperPath,
    int? mtu,
    int? testConcurrency,
    int? healthIntervalSec,
    String? updateChannel,
    bool? updateAutoCheck,
    String? splitMode,
    List<String>? splitApps,
  }) =>
      AppSettings(
        autoConnect: autoConnect ?? this.autoConnect,
        autoMode: autoMode ?? this.autoMode,
        routingMode: routingMode ?? this.routingMode,
        dnsMode: dnsMode ?? this.dnsMode,
        tunEnabled: tunEnabled ?? this.tunEnabled,
        killSwitch: killSwitch ?? this.killSwitch,
        mixedPort: mixedPort ?? this.mixedPort,
        testTimeoutMs: testTimeoutMs ?? this.testTimeoutMs,
        updateOnStart: updateOnStart ?? this.updateOnStart,
        theme: theme ?? this.theme,
        urlTestIntervalMin: urlTestIntervalMin ?? this.urlTestIntervalMin,
        connectionTestUrl: connectionTestUrl ?? this.connectionTestUrl,
        corePath: corePath ?? this.corePath,
        helperPath: helperPath ?? this.helperPath,
        mtu: mtu ?? this.mtu,
        testConcurrency: testConcurrency ?? this.testConcurrency,
        healthIntervalSec: healthIntervalSec ?? this.healthIntervalSec,
        updateChannel: updateChannel ?? this.updateChannel,
        updateAutoCheck: updateAutoCheck ?? this.updateAutoCheck,
        splitMode: splitMode ?? this.splitMode,
        splitApps: splitApps ?? this.splitApps,
      );

  factory AppSettings.fromJson(Map<String, dynamic> j) => AppSettings(
        autoConnect: j['autoConnect'] as bool? ?? false,
        autoMode: j['autoMode'] as bool? ?? true,
        routingMode: j['routingMode'] as String? ?? 'global',
        dnsMode: j['dnsMode'] as String? ?? 'proxy-aware',
        tunEnabled: j['tunEnabled'] as bool? ?? true,
        killSwitch: j['killSwitch'] as bool? ?? false,
        mixedPort: (j['mixedPort'] as num?)?.toInt() ?? 10808,
        testTimeoutMs: (j['testTimeoutMs'] as num?)?.toInt() ?? 5000,
        updateOnStart: j['updateOnStart'] as bool? ?? true,
        theme: j['theme'] as String? ?? 'dark',
        urlTestIntervalMin:
            (j['urlTestIntervalMin'] as num?)?.toInt() ?? 10,
        connectionTestUrl: j['connectionTestUrl'] as String? ??
            'http://connectivitycheck.gstatic.com/generate_204',
        clashApiPort: (j['clashApiPort'] as num?)?.toInt() ?? 16756,
        corePath: j['corePath'] as String? ?? '',
        helperPath: j['helperPath'] as String? ?? '',
        mtu: (j['mtu'] as num?)?.toInt() ?? 9000,
        testConcurrency: (j['testConcurrency'] as num?)?.toInt() ?? 5,
        healthIntervalSec: (j['healthIntervalSec'] as num?)?.toInt() ?? 30,
        updateChannel: j['updateChannel'] as String? ?? 'stable',
        updateAutoCheck: j['updateAutoCheck'] as bool? ?? true,
        splitMode: j['splitMode'] as String? ?? 'off',
        splitApps: [
          for (final e in (j['splitApps'] as List? ?? [])) e.toString()
        ],
      );

  Map<String, dynamic> toJson() => {
        'autoConnect': autoConnect,
        'autoMode': autoMode,
        'routingMode': routingMode,
        'dnsMode': dnsMode,
        'tunEnabled': tunEnabled,
        'killSwitch': killSwitch,
        'mixedPort': mixedPort,
        'testTimeoutMs': testTimeoutMs,
        'updateOnStart': updateOnStart,
        'theme': theme,
        'urlTestIntervalMin': urlTestIntervalMin,
        'connectionTestUrl': connectionTestUrl,
        'clashApiPort': clashApiPort,
        'corePath': corePath,
        'helperPath': helperPath,
        'mtu': mtu,
        'testConcurrency': testConcurrency,
        'healthIntervalSec': healthIntervalSec,
        'updateChannel': updateChannel,
        'updateAutoCheck': updateAutoCheck,
        'splitMode': splitMode,
        'splitApps': splitApps,
      };
}
