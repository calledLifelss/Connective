import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../models/server.dart';
import '../state/app_store.dart';

/// Server add/edit dialog (§8): progressive disclosure — basic fields
/// first, protocol details collapsed in an expansion tile.
Future<void> showServerEditor(
    BuildContext context, AppStore store, Server? existing) async {
  final isNew = existing == null;
  final name = TextEditingController(text: existing?.name ?? '');
  final address =
      TextEditingController(text: existing?.address ?? '');
  final port = TextEditingController(
      text: existing != null ? '${existing.port}' : '443');
  var protocol = existing?.protocol ?? 'vless';
  var transport = existing?.transport ?? 'tcp';
  var security = existing?.security ?? 'none';
  final uuid = TextEditingController(text: existing?.uuid ?? '');
  final password =
      TextEditingController(text: existing?.password ?? '');
  final sni = TextEditingController(text: existing?.sni ?? '');
  final fp =
      TextEditingController(text: existing?.fingerprint ?? '');

  await showDialog(
    context: context,
    builder: (c) => StatefulBuilder(
      builder: (c, setState) => AlertDialog(
        title: Text(isNew ? 'Add server' : 'Edit server'),
        content: SizedBox(
          width: 420,
          child: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                TextField(
                    controller: name,
                    decoration:
                        const InputDecoration(labelText: 'Name')),
                const SizedBox(height: 8),
                Row(
                  children: [
                    Expanded(
                        flex: 3,
                        child: TextField(
                            controller: address,
                            decoration: const InputDecoration(
                                labelText: 'Address'))),
                    const SizedBox(width: 8),
                    Expanded(
                        child: TextField(
                            controller: port,
                            keyboardType: TextInputType.number,
                            decoration: const InputDecoration(
                                labelText: 'Port'))),
                  ],
                ),
                const SizedBox(height: 8),
                Row(
                  children: [
                    Expanded(
                      child: DropdownButtonFormField<String>(
                        initialValue: protocol,
                        decoration:
                            const InputDecoration(labelText: 'Protocol'),
                        items: const ['vless', 'vmess', 'trojan', 'shadowsocks', 'socks']
                            .map((e) => DropdownMenuItem(
                                value: e, child: Text(e.toUpperCase())))
                            .toList(),
                        onChanged: (v) =>
                            setState(() => protocol = v ?? protocol),
                      ),
                    ),
                    const SizedBox(width: 8),
                    Expanded(
                      child: DropdownButtonFormField<String>(
                        initialValue: transport,
                        decoration:
                            const InputDecoration(labelText: 'Transport'),
                        items: const ['tcp', 'ws', 'grpc', 'h2']
                            .map((e) => DropdownMenuItem(
                                value: e, child: Text(e.toUpperCase())))
                            .toList(),
                        onChanged: (v) =>
                            setState(() => transport = v ?? transport),
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 8),
                DropdownButtonFormField<String>(
                  initialValue: security,
                  decoration:
                      const InputDecoration(labelText: 'Security'),
                  items: const ['none', 'tls', 'reality']
                      .map((e) => DropdownMenuItem(
                          value: e, child: Text(e.toUpperCase())))
                      .toList(),
                  onChanged: (v) =>
                      setState(() => security = v ?? security),
                ),
                ExpansionTile(
                  title: const Text('Authentication & TLS details'),
                  tilePadding: EdgeInsets.zero,
                  children: [
                    TextField(
                        controller: uuid,
                        decoration: const InputDecoration(
                            labelText: 'UUID / ID')),
                    const SizedBox(height: 8),
                    TextField(
                        controller: password,
                        decoration: const InputDecoration(
                            labelText: 'Password')),
                    const SizedBox(height: 8),
                    TextField(
                        controller: sni,
                        decoration:
                            const InputDecoration(labelText: 'SNI')),
                    const SizedBox(height: 8),
                    TextField(
                        controller: fp,
                        decoration: const InputDecoration(
                            labelText: 'Fingerprint')),
                  ],
                ),
              ],
            ),
          ),
        ),
        actions: [
          TextButton(
              onPressed: () => Navigator.pop(c),
              child: const Text('Cancel')),
          FilledButton(
            onPressed: () {
              final s = Server(
                id: existing?.id ?? '',
                name: name.text.trim().isEmpty
                    ? address.text.trim()
                    : name.text.trim(),
                subscriptionId: existing?.subscriptionId ?? '',
                address: address.text.trim(),
                port: int.tryParse(port.text.trim()) ?? 0,
                protocol: protocol,
                transport: transport,
                security: security,
                uuid: uuid.text.trim(),
                password: password.text,
                sni: sni.text.trim(),
                fingerprint: fp.text.trim(),
                country: existing?.country ?? '',
              );
              Navigator.pop(c);
              store.saveServer(s);
            },
            child: const Text('Save'),
          ),
        ],
      ),
    ),
  );
}

/// Import dialog: paste a share link (§8, §13).
Future<void> showImportDialog(BuildContext context, AppStore store) async {
  final link = TextEditingController();
  await showDialog(
    context: context,
    builder: (c) => AlertDialog(
      title: const Text('Import server'),
      content: SizedBox(
        width: 420,
        child: TextField(
          controller: link,
          maxLines: 3,
          decoration: const InputDecoration(
            labelText: 'Paste share link (vless://, vmess://, …)',
          ),
        ),
      ),
      actions: [
        TextButton(
            onPressed: () => Navigator.pop(c),
            child: const Text('Cancel')),
        FilledButton(
          onPressed: () {
            Navigator.pop(c);
            final l = link.text.trim();
            if (l.isNotEmpty) store.importServer(l);
          },
          child: const Text('Import'),
        ),
      ],
    ),
  );
}

/// Copy helper with snackbar feedback.
Future<void> copyText(
    BuildContext context, String label, String text) async {
  await Clipboard.setData(ClipboardData(text: text));
  if (context.mounted) {
    ScaffoldMessenger.of(context)
        .showSnackBar(SnackBar(content: Text('$label copied')));
  }
}
