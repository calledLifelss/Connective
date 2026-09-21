import 'package:connective/utils/country.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

/// Pure unit tests for country detection + flag asset resolution.
/// No backend, no widgets: detection must never need IO.
void main() {
  group('explicit metadata wins', () {
    test('ISO-2 code passes through', () {
      expect(
          countryCodeOf(country: 'DE', displayName: 'anything'),
          'de');
      expect(
          countryCodeOf(country: 'jp', displayName: 'anything'),
          'jp');
    });

    test('English names resolve', () {
      expect(
          countryCodeOf(
              country: 'Germany', displayName: 'anything'),
          'de');
      expect(
          countryCodeOf(
              country: 'United States', displayName: 'anything'),
          'us');
      expect(
          countryCodeOf(
              country: 'United Kingdom', displayName: 'anything'),
          'gb');
      expect(
          countryCodeOf(
              country: 'Finland', displayName: 'anything'),
          'fi');
      expect(
          countryCodeOf(
              country: 'Netherlands', displayName: 'anything'),
          'nl');
    });

    test('unmappable explicit country never falls back to the name', () {
      // A wrong flag is worse than none.
      expect(
          countryCodeOf(
              country: 'XX', displayName: 'Frankfurt-01'),
          isNull);
    });
  });

  group('documented name fallback', () {
    test('city hints resolve', () {
      expect(
          countryCodeOf(country: '', displayName: 'Frankfurt-01'),
          'de');
      expect(
          countryCodeOf(country: '', displayName: 'Helsinki-02'),
          'fi');
      expect(
          countryCodeOf(country: '', displayName: 'Tokyo-01'),
          'jp');
    });

    test('country names inside display names resolve', () {
      expect(
          countryCodeOf(
              country: '', displayName: 'Germany 5'), 'de');
      expect(
          countryCodeOf(country: '', displayName: 'Japan East'),
          'jp');
    });

    test('generic names stay unknown', () {
      expect(
          countryCodeOf(country: '', displayName: 'Perf-12'),
          isNull);
      expect(
          countryCodeOf(country: '', displayName: 'T1'), isNull);
      expect(
          countryCodeOf(country: '', displayName: ''), isNull);
    });

    test('no substring false positives', () {
      expect(
          countryCodeOf(
              country: '', displayName: 'Nigeria-1'),
          'ng');
      expect(
          countryCodeOf(country: '', displayName: 'Niger-1'),
          'ne');
      expect(
          countryCodeOf(
              country: '', displayName: 'Papua New Guinea'),
          'pg');
      expect(
          countryCodeOf(
              country: '', displayName: 'Equatorial Guinea'),
          'gq');
      expect(
          countryCodeOf(country: '', displayName: 'Guinea'),
          'gn');
    });
  });

  group('flagAssetOf', () {
    test('known codes map to bundled PNGs', () {
      expect(flagAssetOf('de'), 'assets/flags/de.png');
      expect(flagAssetOf('fi'), 'assets/flags/fi.png');
      expect(flagAssetOf('jp'), 'assets/flags/jp.png');
    });

    test('unknown codes map to null (neutral icon)', () {
      expect(flagAssetOf(null), isNull);
      expect(flagAssetOf(''), isNull);
      expect(flagAssetOf('xx'), isNull);
    });

    // The PNGs must actually ship with the app on every platform:
    // load them through the real test asset bundle.
    testWidgets('flag PNGs load from the bundled assets',
        (t) async {
      await t.runAsync(() async {
        for (final code in ['de', 'fi', 'jp', 'us', 'nl']) {
          final data = await rootBundle
              .load('assets/flags/$code.png');
          expect(data.lengthInBytes, greaterThan(100),
              reason: '$code.png must be a real image');
        }
      });
    });
  });
}
