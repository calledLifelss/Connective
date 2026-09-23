import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../models/stats.dart';
import '../state/app_store.dart';
import '../theme/connective_theme.dart';

/// In-app log viewer (§12): timestamp, level, message, filtering,
/// copy, clear, export. Secrets stay redacted by the backend.
class LogsPage extends StatefulWidget {
  final AppStore store;

  const LogsPage({super.key, required this.store});

  @override
  State<LogsPage> createState() => _LogsPageState();
}

class _LogsPageState extends State<LogsPage> {
  String _filter = '';

  @override
  Widget build(BuildContext context) {
    return ListenableBuilder(
      listenable: widget.store,
      builder: (context, _) {
        final store = widget.store;
        final entries = store.visibleLogs
            .where((e) => _filter.isEmpty ||
                e.message
                    .toLowerCase()
                    .contains(_filter.toLowerCase()))
            .toList()
            .reversed
            .toList();
        return Padding(
          padding: const EdgeInsets.all(ConnectiveTheme.pad),
          child: Column(
            children: [
              Row(
                children: [
                  Expanded(
                    child: SearchBar(
                      hintText: 'Filter logs…',
                      leading: const Icon(Icons.search),
                      onChanged: (q) =>
                          setState(() => _filter = q),
                    ),
                  ),
                  const SizedBox(width: 8),
                  DropdownButton<int>(
                    value: store.logMinLevel,
                    items: const [
                      DropdownMenuItem(
                          value: 0, child: Text('All')),
                      DropdownMenuItem(
                          value: 1, child: Text('Info+')),
                      DropdownMenuItem(
                          value: 2, child: Text('Warn+')),
                      DropdownMenuItem(
                          value: 3, child: Text('Errors')),
                    ],
                    onChanged: (v) {
                      store.setLogLevel(v ?? 0);
                    },
                  ),
                  IconButton(
                    icon: const Icon(Icons.copy),
                    tooltip: 'Copy visible',
                    onPressed: () => _copy(entries),
                  ),
                  IconButton(
                    icon: const Icon(Icons.delete_outline),
                    tooltip: 'Clear',
                    onPressed: () => store.clearLogs(),
                  ),
                ],
              ),
              const SizedBox(height: 8),
              Expanded(
                child: Card(
                  child: entries.isEmpty
                      ? const Center(
                          child: Text('No log entries.'))
                      : ListView.builder(
                          itemCount: entries.length,
                          itemBuilder: (c, i) =>
                              _row(entries[i]),
                        ),
                ),
              ),
            ],
          ),
        );
      },
    );
  }

  Widget _row(LogEntry e) {
    final color = switch (e.level) {
      3 => ConnectiveTheme.danger,
      2 => ConnectiveTheme.warning,
      0 => ConnectiveTheme.textMuted,
      _ => ConnectiveTheme.textSecondary,
    };
    return Padding(
      padding: const EdgeInsets.symmetric(
          horizontal: 12, vertical: 3),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 62,
            child: Text(
                '${e.at.hour.toString().padLeft(2, '0')}:${e.at.minute.toString().padLeft(2, '0')}:${e.at.second.toString().padLeft(2, '0')}',
                style: const TextStyle(
                    fontSize: 11,
                    color: ConnectiveTheme.textSecondary,
                    fontFeatures: [
                      FontFeature.tabularFigures()
                    ])),
          ),
          SizedBox(
            width: 52,
            child: Text(e.levelName,
                style: TextStyle(
                    fontSize: 11,
                    color: color,
                    fontWeight: FontWeight.w700)),
          ),
          Expanded(
            child: SelectableText(e.message,
                style: const TextStyle(fontSize: 12)),
          ),
        ],
      ),
    );
  }

  Future<void> _copy(List<LogEntry> entries) async {
    final text = entries
        .map((e) =>
            '${e.at.toIso8601String()} [${e.levelName}] ${e.message}')
        .join('\n');
    await Clipboard.setData(ClipboardData(text: text));
    if (mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
            content:
                Text('${entries.length} entries copied')),
      );
    }
  }
}
