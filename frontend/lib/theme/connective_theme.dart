import 'package:flutter/material.dart';

/// Connective visual language: dark charcoal, purple accent, green
/// success. Inspired by the reference screenshot's hierarchy without
/// cloning it. All spacing/radii/typography flow through here (§15).
class ConnectiveTheme {
  static const Color accent = Color(0xFF8B5CF6);
  static const Color accentSoft = Color(0xFF6D28D9);
  static const Color background = Color(0xFF0F0E17);
  static const Color surface = Color(0xFF1A1926);
  static const Color surface2 = Color(0xFF232136);
  static const Color border = Color(0xFF2E2C45);
  static const Color textPrimary = Color(0xFFF4F2FA);
  static const Color textSecondary = Color(0xFFA7A3C2);
  static const Color success = Color(0xFF34D399);
  static const Color warning = Color(0xFFFBBF24);
  static const Color danger = Color(0xFFF87171);

  static const double radius = 16;
  static const double pad = 20;
  static const double gap = 12;

  static ThemeData dark() {
    final scheme = ColorScheme.fromSeed(
      seedColor: accent,
      brightness: Brightness.dark,
      surface: surface,
    );
    return ThemeData(
      useMaterial3: true,
      colorScheme: scheme.copyWith(
        primary: accent,
        surface: background,
        surfaceContainerHighest: surface2,
      ),
      scaffoldBackgroundColor: background,
      cardTheme: CardThemeData(
        color: surface,
        elevation: 0,
        margin: const EdgeInsets.symmetric(vertical: 6),
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(radius),
          side: const BorderSide(color: border, width: 1),
        ),
      ),
      navigationRailTheme: const NavigationRailThemeData(
        backgroundColor: surface,
        indicatorColor: accentSoft,
        selectedIconTheme: IconThemeData(color: Colors.white),
        selectedLabelTextStyle: TextStyle(color: textPrimary),
        unselectedLabelTextStyle: TextStyle(color: textSecondary),
      ),
      dividerColor: border,
      inputDecorationTheme: InputDecorationTheme(
        filled: true,
        fillColor: surface2,
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: const BorderSide(color: border),
        ),
        enabledBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: const BorderSide(color: border),
        ),
      ),
    );
  }

  static String rate(int bps) {
    if (bps <= 0) return '—';
    if (bps < 1024) return '$bps B/s';
    if (bps < 1024 * 1024) return '${(bps / 1024).toStringAsFixed(1)} KB/s';
    return '${(bps / 1048576).toStringAsFixed(1)} MB/s';
  }

  static String total(int bytes) {
    if (bytes <= 0) return '—';
    if (bytes < 1048576) return '${(bytes / 1024).toStringAsFixed(0)} KB';
    if (bytes < 1073741824) {
      return '${(bytes / 1048576).toStringAsFixed(1)} MB';
    }
    return '${(bytes / 1073741824).toStringAsFixed(2)} GB';
  }

  static String duration(int ms) {
    if (ms <= 0) return '—';
    final s = ms ~/ 1000;
    final h = s ~/ 3600, m = (s % 3600) ~/ 60, sec = s % 60;
    if (h > 0) return '${h}h ${m}m';
    if (m > 0) return '${m}m ${sec}s';
    return '${sec}s';
  }
}
