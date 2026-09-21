import 'package:flutter/material.dart';

import '../theme/connective_theme.dart';

/// Country detection + bundled flag assets.
///
/// Flag artwork: `flag-icons` 4:3 artwork (MIT, © Panayiotis Lipiridis),
/// rasterized locally to 66×48 PNGs and bundled under
/// `assets/flags/<iso2>.png`, so rendering is identical on
/// Fedora/KDE/Wayland and Windows. No OS emoji, no network loads;
/// Flutter's image cache keeps 500–1000+ rows responsive.
/// See `docs/THIRD_PARTY_NOTICES.md`.
///
/// Detection priority (spec §38):
/// 1. Explicit `Server.country` metadata (ISO-2 code or English name).
/// 2. Documented name fallback: country/city keywords in the display
///    name (e.g. "Frankfurt-01" → DE). This is a heuristic, applied ONLY
///    when explicit metadata is absent.
/// 3. Otherwise unknown → neutral placeholder icon, never a wrong flag.
///
/// No external geolocation is performed; results are memoized in-memory.
String? countryCodeOf({required String country, required String displayName}) {
  final key = '${country.toLowerCase()}\u0000${displayName.toLowerCase()}';
  return _cache.putIfAbsent(key, () {
    final explicit = country.trim();
    if (explicit.isNotEmpty) {
      // Explicit metadata wins; if we cannot map it, return null rather
      // than guessing from the name (never show a wrong flag).
      return _lookup(_normalize(explicit));
    }
    return _lookup(_normalize(displayName));
  });
}

final Map<String, String?> _cache = {};

/// Asset path for a resolved ISO-2 code, or null when unknown.
String? flagAssetOf(String? code) {
  if (code == null || code.isEmpty) return null;
  if (!_kNames.containsKey(code)) return null;
  return 'assets/flags/$code.png';
}

String _normalize(String s) {
  var out = s.toLowerCase();
  const diacritics = {
    'ä': 'a',
    'ö': 'o',
    'ü': 'u',
    'ß': 'ss',
    'é': 'e',
    'è': 'e',
    'ê': 'e',
    'á': 'a',
    'à': 'a',
    'â': 'a',
    'ã': 'a',
    'í': 'i',
    'ì': 'i',
    'î': 'i',
    'ó': 'o',
    'ò': 'o',
    'ô': 'o',
    'õ': 'o',
    'ú': 'u',
    'ù': 'u',
    'û': 'u',
    'ç': 'c',
    'ñ': 'n',
    'å': 'a',
    'ø': 'o',
    'æ': 'ae',
    'ž': 'z',
    'š': 's',
    'č': 'c',
    'ć': 'c',
    'ł': 'l',
    'ı': 'i',
    'ğ': 'g',
    'ș': 's',
    'ț': 't',
  };
  diacritics.forEach((k, v) => out = out.replaceAll(k, v));
  out = out.replaceAll(RegExp(r'[^a-z0-9 ]'), ' ');
  return out.replaceAll(RegExp(r'\s+'), ' ').trim();
}

String? _lookup(String text) {
  if (text.isEmpty) return null;
  // Exact ISO-2 code.
  if (text.length == 2 && _kNames.containsKey(text)) return text;
  // Exact alias match (whole string).
  final exact = _kAliases[text];
  if (exact != null) return exact;
  // Keyword scan, longest alias first, on word boundaries, so that e.g.
  // "niger" never matches inside "nigeria" and "guinea" loses to
  // "equatorial guinea" / "papua new guinea".
  for (final alias in _kSortedAliases) {
    final code = _kAliases[alias]!;
    if (RegExp('\\b${RegExp.escape(alias)}\\b').hasMatch(text)) {
      return code;
    }
  }
  return null;
}

/// Small bundled flag with rounded corners; neutral icon when unknown.
/// Used identically on Dashboard, Servers page, details and status.
class CountryFlag extends StatelessWidget {
  final String? code;
  final double width;
  final double height;

  const CountryFlag({super.key, required this.code, this.width = 22, this.height = 16});

  @override
  Widget build(BuildContext context) {
    final path = flagAssetOf(code);
    if (path == null) {
      return const Icon(Icons.public,
          size: 20, color: ConnectiveTheme.textSecondary);
    }
    return ClipRRect(
      borderRadius: BorderRadius.circular(3),
      child: Image.asset(
        path,
        width: width,
        height: height,
        fit: BoxFit.cover,
        // A missing asset must never break a server row: fall back to
        // the neutral icon instead of the framework error box.
        errorBuilder: (_, __, ___) => const Icon(Icons.public,
            size: 20, color: ConnectiveTheme.textSecondary),
      ),
    );
  }
}

/// ISO-2 → English short name (also the set of bundled assets).
const Map<String, String> _kNames = {
  'ad': 'andorra',
  'ae': 'united arab emirates',
  'af': 'afghanistan',
  'ag': 'antigua and barbuda',
  'ai': 'anguilla',
  'al': 'albania',
  'am': 'armenia',
  'ao': 'angola',
  'aq': 'antarctica',
  'ar': 'argentina',
  'as': 'american samoa',
  'at': 'austria',
  'au': 'australia',
  'aw': 'aruba',
  'ax': 'aland islands',
  'az': 'azerbaijan',
  'ba': 'bosnia and herzegovina',
  'bb': 'barbados',
  'bd': 'bangladesh',
  'be': 'belgium',
  'bf': 'burkina faso',
  'bg': 'bulgaria',
  'bh': 'bahrain',
  'bi': 'burundi',
  'bj': 'benin',
  'bl': 'saint barthelemy',
  'bm': 'bermuda',
  'bn': 'brunei',
  'bo': 'bolivia',
  'bq': 'bonaire',
  'br': 'brazil',
  'bs': 'bahamas',
  'bt': 'bhutan',
  'bw': 'botswana',
  'by': 'belarus',
  'bz': 'belize',
  'ca': 'canada',
  'cc': 'cocos islands',
  'cd': 'democratic republic of the congo',
  'cf': 'central african republic',
  'cg': 'republic of the congo',
  'ch': 'switzerland',
  'ci': 'cote divoire',
  'ck': 'cook islands',
  'cl': 'chile',
  'cm': 'cameroon',
  'cn': 'china',
  'co': 'colombia',
  'cr': 'costa rica',
  'cu': 'cuba',
  'cv': 'cape verde',
  'cw': 'curacao',
  'cx': 'christmas island',
  'cy': 'cyprus',
  'cz': 'czechia',
  'de': 'germany',
  'dj': 'djibouti',
  'dk': 'denmark',
  'dm': 'dominica',
  'do': 'dominican republic',
  'dz': 'algeria',
  'ec': 'ecuador',
  'ee': 'estonia',
  'eg': 'egypt',
  'er': 'eritrea',
  'es': 'spain',
  'et': 'ethiopia',
  'eu': 'european union',
  'fi': 'finland',
  'fj': 'fiji',
  'fk': 'falkland islands',
  'fm': 'micronesia',
  'fo': 'faroe islands',
  'fr': 'france',
  'ga': 'gabon',
  'gb': 'united kingdom',
  'gd': 'grenada',
  'ge': 'georgia',
  'gf': 'french guiana',
  'gg': 'guernsey',
  'gh': 'ghana',
  'gi': 'gibraltar',
  'gl': 'greenland',
  'gm': 'gambia',
  'gn': 'guinea',
  'gp': 'guadeloupe',
  'gq': 'equatorial guinea',
  'gr': 'greece',
  'gt': 'guatemala',
  'gu': 'guam',
  'gw': 'guinea bissau',
  'gy': 'guyana',
  'hk': 'hong kong',
  'hn': 'honduras',
  'hr': 'croatia',
  'ht': 'haiti',
  'hu': 'hungary',
  'id': 'indonesia',
  'ie': 'ireland',
  'il': 'israel',
  'im': 'isle of man',
  'in': 'india',
  'iq': 'iraq',
  'ir': 'iran',
  'is': 'iceland',
  'it': 'italy',
  'je': 'jersey',
  'jm': 'jamaica',
  'jo': 'jordan',
  'jp': 'japan',
  'ke': 'kenya',
  'kg': 'kyrgyzstan',
  'kh': 'cambodia',
  'ki': 'kiribati',
  'km': 'comoros',
  'kn': 'saint kitts and nevis',
  'kp': 'north korea',
  'kr': 'south korea',
  'kw': 'kuwait',
  'ky': 'cayman islands',
  'kz': 'kazakhstan',
  'la': 'laos',
  'lb': 'lebanon',
  'lc': 'saint lucia',
  'li': 'liechtenstein',
  'lk': 'sri lanka',
  'lr': 'liberia',
  'ls': 'lesotho',
  'lt': 'lithuania',
  'lu': 'luxembourg',
  'lv': 'latvia',
  'ly': 'libya',
  'ma': 'morocco',
  'mc': 'monaco',
  'md': 'moldova',
  'me': 'montenegro',
  'mg': 'madagascar',
  'mh': 'marshall islands',
  'mk': 'north macedonia',
  'ml': 'mali',
  'mm': 'myanmar',
  'mn': 'mongolia',
  'mo': 'macao',
  'mp': 'northern mariana islands',
  'mq': 'martinique',
  'mr': 'mauritania',
  'ms': 'montserrat',
  'mt': 'malta',
  'mu': 'mauritius',
  'mv': 'maldives',
  'mw': 'malawi',
  'mx': 'mexico',
  'my': 'malaysia',
  'mz': 'mozambique',
  'na': 'namibia',
  'nc': 'new caledonia',
  'ne': 'niger',
  'ng': 'nigeria',
  'ni': 'nicaragua',
  'nl': 'netherlands',
  'no': 'norway',
  'np': 'nepal',
  'nr': 'nauru',
  'nz': 'new zealand',
  'om': 'oman',
  'pa': 'panama',
  'pe': 'peru',
  'pf': 'french polynesia',
  'pg': 'papua new guinea',
  'ph': 'philippines',
  'pk': 'pakistan',
  'pl': 'poland',
  'pr': 'puerto rico',
  'ps': 'palestine',
  'pt': 'portugal',
  'pw': 'palau',
  'py': 'paraguay',
  'qa': 'qatar',
  're': 'reunion',
  'ro': 'romania',
  'rs': 'serbia',
  'ru': 'russia',
  'rw': 'rwanda',
  'sa': 'saudi arabia',
  'sb': 'solomon islands',
  'sc': 'seychelles',
  'sd': 'sudan',
  'se': 'sweden',
  'sg': 'singapore',
  'si': 'slovenia',
  'sk': 'slovakia',
  'sl': 'sierra leone',
  'sm': 'san marino',
  'sn': 'senegal',
  'so': 'somalia',
  'sr': 'suriname',
  'ss': 'south sudan',
  'sv': 'el salvador',
  'sx': 'sint maarten',
  'sy': 'syria',
  'sz': 'eswatini',
  'tc': 'turks and caicos islands',
  'td': 'chad',
  'tg': 'togo',
  'th': 'thailand',
  'tj': 'tajikistan',
  'tl': 'timor leste',
  'tm': 'turkmenistan',
  'tn': 'tunisia',
  'to': 'tonga',
  'tr': 'turkey',
  'tt': 'trinidad and tobago',
  'tv': 'tuvalu',
  'tw': 'taiwan',
  'tz': 'tanzania',
  'ua': 'ukraine',
  'ug': 'uganda',
  'us': 'united states',
  'uy': 'uruguay',
  'uz': 'uzbekistan',
  'va': 'vatican city',
  'vc': 'saint vincent and the grenadines',
  've': 'venezuela',
  'vg': 'british virgin islands',
  'vi': 'us virgin islands',
  'vn': 'vietnam',
  'vu': 'vanuatu',
  'ws': 'samoa',
  'ye': 'yemen',
  'za': 'south africa',
  'zm': 'zambia',
  'zw': 'zimbabwe',
};

/// Alias/city → ISO-2. Merged with [_kNames] at startup; sorted
/// longest-first for the keyword scan in [_lookup].
final Map<String, String> _kAliases = {
  for (final e in _kNames.entries) e.value: e.key,
  // Common English aliases.
  'usa': 'us',
  'america': 'us',
  'uk': 'gb',
  'england': 'gb',
  'great britain': 'gb',
  'britain': 'gb',
  'northern ireland': 'gb',
  'scotland': 'gb',
  'wales': 'gb',
  'uae': 'ae',
  'emirates': 'ae',
  'dubai': 'ae',
  'korea': 'kr',
  'czech republic': 'cz',
  'turkiye': 'tr',
  'macau': 'mo',
  'hongkong': 'hk',
  'swaziland': 'sz',
  'cape verde': 'cv',
  'cabo verde': 'cv',
  'east timor': 'tl',
  'vatican': 'va',
  'holy see': 'va',
  'ivory coast': 'ci',
  'congo': 'cd',
  'dr congo': 'cd',
  'congo kinshasa': 'cd',
  'congo brazzaville': 'cg',
  'burma': 'mm',
  'macedonia': 'mk',
  'palestine state': 'ps',
  'russian federation': 'ru',
  'moldavia': 'md',
  'lao': 'la',
  'syrian arab republic': 'sy',
  'tanzania united republic': 'tz',
  'venezuela bolivarian republic': 've',
  'bolivia plurinational state': 'bo',
  'iran islamic republic': 'ir',
  'persia': 'ir',
  // German cities/aliases.
  'deutschland': 'de',
  'berlin': 'de',
  'frankfurt': 'de',
  'munich': 'de',
  'munchen': 'de',
  'nuremberg': 'de',
  'hamburg': 'de',
  // Other major VPN PoP cities.
  'helsinki': 'fi',
  'tokyo': 'jp',
  'osaka': 'jp',
  'paris': 'fr',
  'amsterdam': 'nl',
  'holland': 'nl',
  'london': 'gb',
  'manchester': 'gb',
  'stockholm': 'se',
  'zurich': 'ch',
  'geneva': 'ch',
  'new york': 'us',
  'ashburn': 'us',
  'dallas': 'us',
  'los angeles': 'us',
  'miami': 'us',
  'chicago': 'us',
  'seattle': 'us',
  'san jose': 'us',
  'atlanta': 'us',
  'toronto': 'ca',
  'montreal': 'ca',
  'vancouver': 'ca',
  'sydney': 'au',
  'melbourne': 'au',
  'mumbai': 'in',
  'delhi': 'in',
  'chennai': 'in',
  'sao paulo': 'br',
  'madrid': 'es',
  'barcelona': 'es',
  'milan': 'it',
  'rome': 'it',
  'warsaw': 'pl',
  'vienna': 'at',
  'brussels': 'be',
  'oslo': 'no',
  'copenhagen': 'dk',
  'dublin': 'ie',
  'lisbon': 'pt',
  'athens': 'gr',
  'bucharest': 'ro',
  'budapest': 'hu',
  'belgrade': 'rs',
  'zagreb': 'hr',
  'sofia': 'bg',
  'prague': 'cz',
  'bratislava': 'sk',
  'ljubljana': 'si',
  'tallinn': 'ee',
  'riga': 'lv',
  'vilnius': 'lt',
  'kyiv': 'ua',
  'kiev': 'ua',
  'moscow': 'ru',
  'saint petersburg': 'ru',
  'istanbul': 'tr',
  'tel aviv': 'il',
  'cairo': 'eg',
  'johannesburg': 'za',
  'cape town': 'za',
  'lagos': 'ng',
  'nairobi': 'ke',
  'buenos aires': 'ar',
  'santiago': 'cl',
  'bogota': 'co',
  'lima': 'pe',
  'mexico city': 'mx',
  'bangkok': 'th',
  'hanoi': 'vn',
  'ho chi minh': 'vn',
  'saigon': 'vn',
  'kuala lumpur': 'my',
  'jakarta': 'id',
  'manila': 'ph',
  'taipei': 'tw',
  'seoul': 'kr',
  'auckland': 'nz',
  'reykjavik': 'is',
  'tbilisi': 'ge',
  'yerevan': 'am',
  'baku': 'az',
  'almaty': 'kz',
  'astana': 'kz',
  'tashkent': 'uz',
  'karachi': 'pk',
  'lahore': 'pk',
  'dhaka': 'bd',
  'colombo': 'lk',
  'kathmandu': 'np',
  'phnom penh': 'kh',
  'yangon': 'mm',
  'doha': 'qa',
  'riyadh': 'sa',
  'jeddah': 'sa',
  'amman': 'jo',
  'beirut': 'lb',
  'baghdad': 'iq',
  'tehran': 'ir',
  'accra': 'gh',
  'dakar': 'sn',
  'nicosia': 'cy',
  'valletta': 'mt',
  'monaco': 'mc',
  'andorra la vella': 'ad',
  'reykjavik city': 'is',
};

final List<String> _kSortedAliases = (() {
  final keys = _kAliases.keys.toList()
    ..sort((a, b) => b.length.compareTo(a.length));
  return keys;
})();
