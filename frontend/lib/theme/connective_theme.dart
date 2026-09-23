import 'package:flutter/material.dart';

/// Connective visual language: restrained dark desktop utility.
///
/// Charcoal surfaces, thin gray borders, one sparingly-used green
/// accent. No neon, no gradients, no glows. All spacing/radii/
/// typography flow through here.
class ConnectiveTheme {
  // Core surfaces.
  static const Color background = Color(0xFF111318);
  static const Color surface = Color(0xFF181C22);
  static const Color surfaceElevated = Color(0xFF20252D);
  static const Color border = Color(0xFF2D333D);
  static const Color borderSubtle = Color(0xFF232932);

  // Text: three-step hierarchy — bright for primary info, mid gray
  // for secondary, dark gray for metadata only.
  static const Color textPrimary = Color(0xFFE8EBEF);
  static const Color textSecondary = Color(0xFFA2A9B4);
  static const Color textMuted = Color(0xFF6E7580);

  // Semantic accent — used sparingly (primary actions, active state).
  static const Color accent = Color(0xFF27D7A0);
  static const Color accentHover = Color(0xFF35E0AB);
  static const Color accentDim = Color(0xFF1B3A32);
  static const Color onAccent = Color(0xFF0B1512);

  // Status.
  static const Color success = Color(0xFF27D7A0);
  static const Color warning = Color(0xFFD9A441);
  static const Color danger = Color(0xFFE05C67);
  static const Color info = Color(0xFF6F8DAF);

  // Legacy aliases kept so call sites don't churn:
  // surface2 -> elevated surface, accentSoft -> dim green wash.
  static const Color surface2 = surfaceElevated;
  static const Color accentSoft = accentDim;

  static const double radius = 8;
  static const double radiusSmall = 6;
  static const double pad = 16;
  static const double gap = 8;

  static ThemeData dark() {
    final scheme = const ColorScheme.dark(
      primary: accent,
      onPrimary: onAccent,
      secondary: info,
      onSecondary: textPrimary,
      surface: background,
      onSurface: textPrimary,
      surfaceContainerHighest: surfaceElevated,
      error: danger,
      onError: textPrimary,
      outline: border,
    );
    return ThemeData(
      useMaterial3: true,
      colorScheme: scheme,
      scaffoldBackgroundColor: background,
      dividerColor: borderSubtle,
      splashFactory: NoSplash.splashFactory,
      textTheme: const TextTheme(
        headlineSmall: TextStyle(
            fontSize: 18, fontWeight: FontWeight.w700, color: textPrimary),
        titleMedium: TextStyle(
            fontSize: 14, fontWeight: FontWeight.w600, color: textPrimary),
        titleSmall: TextStyle(
            fontSize: 13, fontWeight: FontWeight.w600, color: textPrimary),
        bodyMedium: TextStyle(fontSize: 13, color: textPrimary, height: 1.4),
        bodySmall:
            TextStyle(fontSize: 12, color: textSecondary, height: 1.4),
      ),
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
        selectedIconTheme: IconThemeData(color: accent, size: 20),
        unselectedIconTheme:
            IconThemeData(color: textSecondary, size: 20),
        selectedLabelTextStyle:
            TextStyle(color: textPrimary, fontSize: 12),
        unselectedLabelTextStyle:
            TextStyle(color: textSecondary, fontSize: 12),
      ),
      dividerTheme: const DividerThemeData(
          color: borderSubtle, thickness: 1, space: 1),
      inputDecorationTheme: InputDecorationTheme(
        filled: true,
        fillColor: surfaceElevated,
        hintStyle: const TextStyle(color: textMuted, fontSize: 13),
        labelStyle: const TextStyle(color: textSecondary, fontSize: 13),
        contentPadding:
            const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(radiusSmall),
          borderSide: const BorderSide(color: border),
        ),
        enabledBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(radiusSmall),
          borderSide: const BorderSide(color: border),
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(radiusSmall),
          borderSide: const BorderSide(color: info, width: 1.2),
        ),
      ),
      searchBarTheme: SearchBarThemeData(
        backgroundColor:
            WidgetStateProperty.all(surfaceElevated),
        side: WidgetStateProperty.all(
            const BorderSide(color: border)),
        shape: WidgetStateProperty.all(RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(radiusSmall))),
        hintStyle: WidgetStateProperty.all(
            const TextStyle(color: textMuted, fontSize: 13)),
        textStyle: WidgetStateProperty.all(
            const TextStyle(color: textPrimary, fontSize: 13)),
        elevation: WidgetStateProperty.all(0),
        // Desktop-density search field, not a tall website input.
        constraints: const BoxConstraints(minHeight: 40),
      ),
      filledButtonTheme: FilledButtonThemeData(
        style: ButtonStyle(
          backgroundColor: WidgetStateProperty.resolveWith((states) {
            if (states.contains(WidgetState.disabled)) {
              return const Color(0xFF2A3038);
            }
            if (states.contains(WidgetState.hovered) ||
                states.contains(WidgetState.pressed)) {
              return accentHover;
            }
            return accent;
          }),
          foregroundColor: WidgetStateProperty.resolveWith((states) {
            if (states.contains(WidgetState.disabled)) {
              return textMuted;
            }
            return onAccent;
          }),
          textStyle: WidgetStateProperty.all(const TextStyle(
              fontSize: 13, fontWeight: FontWeight.w600)),
          padding: WidgetStateProperty.all(const EdgeInsets.symmetric(
              horizontal: 16, vertical: 12)),
          minimumSize:
              WidgetStateProperty.all(const Size(96, 38)),
          shape: WidgetStateProperty.all(RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(radiusSmall))),
          elevation: WidgetStateProperty.all(0),
        ),
      ),
      outlinedButtonTheme: OutlinedButtonThemeData(
        style: ButtonStyle(
          foregroundColor:
              WidgetStateProperty.all(textPrimary),
          side: WidgetStateProperty.all(
              const BorderSide(color: border)),
          textStyle: WidgetStateProperty.all(const TextStyle(
              fontSize: 13, fontWeight: FontWeight.w600)),
          padding: WidgetStateProperty.all(const EdgeInsets.symmetric(
              horizontal: 14, vertical: 10)),
          shape: WidgetStateProperty.all(RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(radiusSmall))),
        ),
      ),
      textButtonTheme: TextButtonThemeData(
        style: ButtonStyle(
          foregroundColor: WidgetStateProperty.resolveWith((states) {
            if (states.contains(WidgetState.disabled)) {
              return textMuted;
            }
            return accent;
          }),
          textStyle: WidgetStateProperty.all(const TextStyle(
              fontSize: 13, fontWeight: FontWeight.w600)),
          padding: WidgetStateProperty.all(const EdgeInsets.symmetric(
              horizontal: 10, vertical: 8)),
          shape: WidgetStateProperty.all(RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(radiusSmall))),
        ),
      ),
      iconButtonTheme: IconButtonThemeData(
        style: ButtonStyle(
          foregroundColor:
              WidgetStateProperty.all(textSecondary),
          iconSize: WidgetStateProperty.all(18),
        ),
      ),
      switchTheme: SwitchThemeData(
        thumbColor: WidgetStateProperty.resolveWith((states) {
          if (states.contains(WidgetState.selected)) {
            return onAccent;
          }
          return textSecondary;
        }),
        trackColor: WidgetStateProperty.resolveWith((states) {
          if (states.contains(WidgetState.selected)) {
            return accent;
          }
          return const Color(0xFF2A3038);
        }),
        trackOutlineColor:
            WidgetStateProperty.all(Colors.transparent),
      ),
      radioTheme: RadioThemeData(
        fillColor: WidgetStateProperty.resolveWith((states) {
          if (states.contains(WidgetState.selected)) {
            return accent;
          }
          return textSecondary;
        }),
      ),
      progressIndicatorTheme: const ProgressIndicatorThemeData(
        color: accent,
        linearTrackColor: Color(0xFF262C35),
        circularTrackColor: Color(0xFF262C35),
        linearMinHeight: 5,
      ),
      snackBarTheme: SnackBarThemeData(
        backgroundColor: surfaceElevated,
        contentTextStyle:
            const TextStyle(color: textPrimary, fontSize: 13),
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(radiusSmall),
          side: const BorderSide(color: border),
        ),
        behavior: SnackBarBehavior.floating,
      ),
      dialogTheme: DialogThemeData(
        backgroundColor: surface,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(10),
          side: const BorderSide(color: border),
        ),
        titleTextStyle: const TextStyle(
            color: textPrimary,
            fontSize: 15,
            fontWeight: FontWeight.w600),
        contentTextStyle: const TextStyle(
            color: textSecondary, fontSize: 13, height: 1.45),
      ),
      popupMenuTheme: PopupMenuThemeData(
        color: surfaceElevated,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(radiusSmall),
          side: const BorderSide(color: border),
        ),
        textStyle:
            const TextStyle(color: textPrimary, fontSize: 13),
      ),
      tooltipTheme: TooltipThemeData(
        decoration: BoxDecoration(
          color: surfaceElevated,
          borderRadius: BorderRadius.circular(radiusSmall),
          border: Border.all(color: border),
        ),
        textStyle:
            const TextStyle(color: textPrimary, fontSize: 12),
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
