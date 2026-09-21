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

  String get displayName => customName.isNotEmpty ? customName : name;

  /// Compact protocol line for collapsed rows, e.g. "VLESS / WS / TLS".
  String get compactInfo {
    final parts = <String>[protocol.toUpperCase()];
    if (transport != 'tcp') parts.add(transport.toUpperCase());
    if (security != 'none') parts.add(security.toUpperCase());
    return parts.join(' / ');
  }

  String get latencyLabel => latencyMs < 0 ? 'n/a' : '${latencyMs}ms';

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
