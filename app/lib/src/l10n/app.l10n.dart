import 'dart:async';

import 'package:flutter/material.dart';
import 'package:intl/intl.dart';

// This file was generated in two steps, using the Dart intl tools. With the
// app's root directory (the one that contains pubspec.yaml) as the current
// directory:
//
// flutter pub get
// flutter pub pub run intl_translation:extract_to_arb --output-dir=lib/src/l10n lib/src/l10n/app.l10n.dart
// flutter pub pub run intl_translation:generate_from_arb --output-dir=lib/src/l10n --no-use-deferred-loading lib/src/l10n/app.l10n.dart lib/src/l10n/intl_*.arb
//
// The second command generates intl_messages.arb and the third generates
// messages_all.dart. There's more about this process in
// https://pub.dartlang.org/packages/intl.
import './messages_all.dart';

typedef OnAppLocalizationsLoad = Function(AppLocalizations l10n);

class AppLocalizations {
  static final List<OnAppLocalizationsLoad> _onLoads = [];
  static void addOnLoad(OnAppLocalizationsLoad onLoad) => _onLoads.add(onLoad);
  static bool removeOnLoad(OnAppLocalizationsLoad onLoad) =>
      _onLoads.remove(onLoad);

  static Future<AppLocalizations> load(Locale locale) {
    final String name =
        locale.countryCode.isEmpty ? locale.languageCode : locale.toString();
    final String localeName = Intl.canonicalizedLocale(name);

    return initializeMessages(localeName).then((_) {
      Intl.defaultLocale = localeName;
      final l10n = AppLocalizations();
      for (final onLoad in _onLoads) {
        onLoad(l10n);
      }
      return l10n;
    });
  }

  static AppLocalizations of(BuildContext context) {
    return Localizations.of<AppLocalizations>(context, AppLocalizations);
  }

  String get notificationChannelName {
    return Intl.message(
      'Hybrid',
      name: 'notificationChannelName',
      desc: 'forground channel name',
    );
  }

  String get notificationChannelDesc {
    return Intl.message(
      'Keep hybrid always running',
      name: 'notificationChannelDesc',
      desc: 'forground channel description',
    );
  }

  String get serviceTitle {
    return Intl.message(
      'Hybrid',
      name: 'serviceTitle',
      desc: 'forground service title',
    );
  }

  String get nodeRunning {
    return Intl.message(
      'Node is running',
      name: 'nodeRunning',
      desc: 'node status text, showed in notification',
    );
  }

  String get nodeStopped {
    return Intl.message(
      'Node stopped',
      name: 'nodeStopped',
      desc: 'node status text, showed in notification',
    );
  }

  String get nodeError {
    return Intl.message(
      'Node stopped with error',
      name: 'nodeError',
      desc: 'node status text, showed in notification',
    );
  }

  String get appName {
    return Intl.message(
      'Hybrid',
      name: 'appName',
    );
  }

  String get appDesc {
    return Intl.message(
      'Decentralized access anywhere.',
      name: 'appDesc',
    );
  }

  String get home {
    return Intl.message(
      'Home',
      name: 'home',
    );
  }

  String get configure {
    return Intl.message(
      'Configure',
      name: 'configure',
    );
  }

  String get about {
    return Intl.message(
      'About',
      name: 'about',
    );
  }

  String get devModeLabel {
    return Intl.message(
      'Dev',
      name: 'devModeLabel',
    );
  }

  String get configureBackAlertTitle {
    return Intl.message(
      'Alert',
      name: 'configureBackAlertTitle',
    );
  }

  String get configureBackAlertContent {
    return Intl.message(
      'Are you sure go back without save to stage.',
      name: 'configureBackAlertContent',
    );
  }

  String get configureBackAlertStay {
    return Intl.message(
      'Stay',
      name: 'configureBackAlertStay',
    );
  }

  String get configureBackAlertGoBack {
    return Intl.message(
      'Go back',
      name: 'configureBackAlertGoBack',
    );
  }

  String get configureBasicTitle {
    return Intl.message(
      'Basic',
      name: 'configureBasicTitle',
    );
  }

  String get configureBasicBindLabel {
    return Intl.message(
      'Bind',
      name: 'configureBasicBindLabel',
    );
  }

  String get configureBasicBindHint {
    return Intl.message(
      'Address that should be set to proxy settings',
      name: 'configureBasicBindHint',
    );
  }

  String get configureBasicBindBadTcpAddr {
    return Intl.message(
      'Accept only tcp addresses',
      name: 'configureBasicBindBadTcpAddr',
    );
  }

  String get configureBasicFlushIntervalLabel {
    return Intl.message(
      'FlushInterval',
      name: 'configureBasicFlushIntervalLabel',
    );
  }

  String get configureBasicFlushIntervalUnitLabel {
    return Intl.message(
      'ms',
      name: 'configureBasicFlushIntervalUnitLabel',
    );
  }

  String get configureBasicFlushIntervalNegtive {
    return Intl.message(
      'Accept positive values',
      name: 'configureBasicFlushIntervalNegtive',
    );
  }

  String get configureBasicFlushIntervalUint32 {
    return Intl.message(
      'Accept uint32 values',
      name: 'configureBasicFlushIntervalUint32',
    );
  }

  String get configureBasicTokenLabel {
    return Intl.message(
      'Token',
      name: 'configureBasicTokenLabel',
    );
  }

  String get configureLogLevelLabel {
    return Intl.message(
      'Level',
      name: 'configureLogLevelLabel',
    );
  }

  String get configureLogLevelEmpty {
    return Intl.message(
      'Must pick a level',
      name: 'configureLogLevelEmpty',
    );
  }

  String get configureLogTargetLabel {
    return Intl.message(
      'Target',
      name: 'configureLogTargetLabel',
    );
  }

  String get configureLogTargetHint {
    return Intl.message(
      '"tcp://host:port?timeout=5s", file, sentryDSN or empty',
      name: 'configureLogTargetHint',
    );
  }
}

class AppLocalizationsDelegate extends LocalizationsDelegate<AppLocalizations> {
  const AppLocalizationsDelegate();

  @override
  bool isSupported(Locale locale) => ['en'].contains(locale.languageCode);

  @override
  Future<AppLocalizations> load(Locale locale) => AppLocalizations.load(locale);

  @override
  bool shouldReload(AppLocalizationsDelegate old) => false;
}
