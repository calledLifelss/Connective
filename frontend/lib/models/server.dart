/// Dart mirrors of backend/internal/servers JSON. Field-for-field copies;
/// the daemon is the source of truth (see docs/IPC_CONTRACT.md).
class Server {
  final String id;
  final String name;
  final String customName;
  final String subscriptionId;
  final String address;
  final int port;
  final String protocol;
  final String transport;
  final String security;
  final String country;
  final bool favorite;
  final int latencyMs; // -1 = never tested; UI must show "n/a", never invent
  final String health;
  final String uuid;
  final String password;
  final String method;
  final String flow;
  final String sni;
  final String fingerprint;
  final String publicKey;
  final String shortId;
  final String path;
  final String host;
  final String serviceName;

  const Server({
    required this.id,
    required this.name,
    this.customName = '',
    this.subscriptionId = '',
    required this.address,
    required this.port,
    required this.protocol,
    this.transport = 'tcp',
    this.security = 'none',
    this.country = '',
    this.favorite = false,
    this.latencyMs = -1,
    this.health = 'unknown',
    this.uuid = '',
    this.password = '',
    this.method = '',
    this.flow = '',
    this.sni = '',
    this.fingerprint = '',
    this.publicKey = '',
    this.shortId = '',
    this.path = '',
    this.host = '',
    this.serviceName = '',
  });

  /// Display name sanitized for presentation: strips flag emoji
  /// (regional-indicator pairs) and invisible format characters that
  /// some subscriptions embed in server names. Flags already render
  /// as real bundled assets, and desktop Linux rarely has an emoji
  /// font — without this the name shows tofu boxes. Raw [name] is
  /// untouched so backend round-trips and editing are unaffected.
  /// Falls back to the raw name if nothing visible remains.
  String get displayName {
    final raw = customName.isNotEmpty ? customName : name;
    final clean = _sanitizeDisplayName(raw);
    return clean.isEmpty ? raw : clean;
  }

  static bool _isDecorative(int rune) {
    // Regional indicators (flag emoji components U+1F1E6–U+1F1FF).
    if (rune >= 0x1F1E6 && rune <= 0x1F1FF) return true;
    // All emoji/pictograph blocks: symbols & pictographs, emoticons,
    // transport & map, alchemical, geometric shapes extended,
    // supplemental symbols, chess, symbols extended-A, etc.
    // Scripts (Latin, Arabic, CJK, …) are never in these ranges.
    if (rune >= 0x1F000 && rune <= 0x1FAFF) return true;
    // Misc symbols (★ ☀ ⚡ …), dingbats (❤ ✨ …), misc symbols &
    // arrows (⭐ …): decorative in server names, often tofu on
    // desktop Linux without emoji fonts.
    if (rune >= 0x2600 && rune <= 0x26FF) return true;
    if (rune >= 0x2700 && rune <= 0x27BF) return true;
    if (rune >= 0x2B00 && rune <= 0x2BFF) return true;
    // Miscellaneous technical symbols (⏳ ⏰ …): decorative in
    // server names, often tofu on desktop Linux without emoji fonts.
    if (rune >= 0x2300 && rune <= 0x23FF) return true;
    // Combining enclosing keycap (e.g. 1️⃣).
    if (rune == 0x20E3) return true;
    // Variation selectors and tag characters.
    if (rune >= 0xFE00 && rune <= 0xFE0F) return true;
    if (rune >= 0xE0020 && rune <= 0xE007F) return true;
    // Zero-width space/non-joiner/joiner and BOM.
    if (rune == 0x200B || rune == 0x200C || rune == 0x200D) return true;
    return rune == 0xFEFF;
  }

  static String _sanitizeDisplayName(String s) {
    final buf = StringBuffer();
    for (final rune in s.runes) {
      if (_isDecorative(rune)) continue;
      buf.writeCharCode(rune);
    }
    return buf.toString().replaceAll(RegExp(r'\s+'), ' ').trim();
  }

  /// Compact protocol line for collapsed rows, e.g. "VLESS / WS / TLS".
  String get compactInfo {
    final parts = <String>[protocol.toUpperCase()];
    if (transport != 'tcp') parts.add(transport.toUpperCase());
    if (security != 'none') parts.add(security.toUpperCase());
    return parts.join(' / ');
  }

  String get latencyLabel => latencyMs < 0 ? 'n/a' : '${latencyMs} ms';

  /// Copy with selective overrides (used for favorite toggling etc.).
  Server copyWith({
    String? id,
    String? name,
    String? customName,
    String? subscriptionId,
    String? address,
    int? port,
    String? protocol,
    String? transport,
    String? security,
    String? country,
    bool? favorite,
    int? latencyMs,
    String? health,
  }) =>
      Server(
        id: id ?? this.id,
        name: name ?? this.name,
        customName: customName ?? this.customName,
        subscriptionId: subscriptionId ?? this.subscriptionId,
        address: address ?? this.address,
        port: port ?? this.port,
        protocol: protocol ?? this.protocol,
        transport: transport ?? this.transport,
        security: security ?? this.security,
        country: country ?? this.country,
        favorite: favorite ?? this.favorite,
        latencyMs: latencyMs ?? this.latencyMs,
        health: health ?? this.health,
        uuid: uuid,
        password: password,
        method: method,
        flow: flow,
        sni: sni,
        fingerprint: fingerprint,
        publicKey: publicKey,
        shortId: shortId,
        path: path,
        host: host,
        serviceName: serviceName,
      );

  /// True for usable paths (healthy or not-yet-tested).
  bool get isUsable => health == 'healthy' || health == 'unknown';

  /// Sort weight for health: healthy first, then unknown/degraded,
  /// unhealthy last.
  int get healthWeight {
    switch (health) {
      case 'healthy':
        return 0;
      case 'unknown':
        return 1;
      case 'degraded':
        return 2;
      default:
        return 3;
    }
  }

  factory Server.fromJson(Map<String, dynamic> j) => Server(
        id: j['id'] as String? ?? '',
        name: j['name'] as String? ?? '',
        customName: j['customName'] as String? ?? '',
        subscriptionId: j['subscriptionId'] as String? ?? '',
        address: j['address'] as String? ?? '',
        port: (j['port'] as num?)?.toInt() ?? 0,
        protocol: j['protocol'] as String? ?? '',
        transport: j['transport'] as String? ?? 'tcp',
        security: j['security'] as String? ?? 'none',
        country: j['country'] as String? ?? '',
        favorite: j['favorite'] as bool? ?? false,
        latencyMs: (j['latencyMs'] as num?)?.toInt() ?? -1,
        health: j['health'] as String? ?? 'unknown',
        uuid: j['uuid'] as String? ?? '',
        password: j['password'] as String? ?? '',
        method: j['method'] as String? ?? '',
        flow: j['flow'] as String? ?? '',
        sni: j['sni'] as String? ?? '',
        fingerprint: j['fingerprint'] as String? ?? '',
        publicKey: j['publicKey'] as String? ?? '',
        shortId: j['shortId'] as String? ?? '',
        path: j['path'] as String? ?? '',
        host: j['host'] as String? ?? '',
        serviceName: j['serviceName'] as String? ?? '',
      );

  Map<String, dynamic> toJson() => {
        'id': id,
        'name': name,
        'customName': customName,
        'subscriptionId': subscriptionId,
        'address': address,
        'port': port,
        'protocol': protocol,
        'transport': transport,
        'security': security,
        'country': country,
        'favorite': favorite,
        'latencyMs': latencyMs,
        'health': health,
        'uuid': uuid,
        'password': password,
        'method': method,
        'flow': flow,
        'sni': sni,
        'fingerprint': fingerprint,
        'publicKey': publicKey,
        'shortId': shortId,
        'path': path,
        'host': host,
        'serviceName': serviceName,
      };

  /// Protocol-dependent detail rows for the expanded view (§17).
  List<MapEntry<String, String>> detailRows() {
    final rows = <MapEntry<String, String>>[
      MapEntry('Protocol', protocol.toUpperCase()),
      MapEntry('Transport', transport),
      MapEntry('Security', security),
      MapEntry('Address', address),
      MapEntry('Port', '$port'),
    ];
    void add(String k, String v) {
      if (v.isNotEmpty) rows.add(MapEntry(k, v));
    }

    add('SNI', sni);
    add('Fingerprint', fingerprint);
    add('Flow', flow);
    add('Reality key', publicKey.isNotEmpty ? '${publicKey.substring(0, publicKey.length.clamp(0, 12))}…' : '');
    add('Short ID', shortId);
    add('Path', path);
    add('Host', host);
    add('Service', serviceName);
    add('Method', method);
    if (subscriptionId.isNotEmpty) {
      rows.add(const MapEntry('Subscription', 'yes'));
    }
    rows.add(MapEntry('Latency', latencyLabel));
    rows.add(MapEntry('Health', health));
    return rows;
  }
}
