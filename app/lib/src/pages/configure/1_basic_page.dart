import 'package:flutter/material.dart';
import 'package:card_settings/card_settings.dart';

import '../../l10n/app.l10n.dart';
import '../../protos.dart';

import './configure_leaf_page.dart';

class ConfigBasicPage extends StatelessWidget {
  static final int _tokenLines = 5;
  static final RegExp ipv4RegExp = RegExp(
      r"^(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$");

  final Config config;

  const ConfigBasicPage({Key key, this.config}) : super(key: key);

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context);

    CardSettings _buildPortraitLayout(bool autoValidate) {
      return CardSettings(
        children: <Widget>[
          // dev
          CardSettingsSwitch(
            label: l10n.configureBasicDevLabel,
            initialValue: config.dev,
            onSaved: (value) => config.dev = value,
          ),
          // bind
          CardSettingsText(
            label: l10n.configureBasicBindLabel,
            hintText: l10n.configureBasicBindHint,
            initialValue: config.bind,
            autovalidate: autoValidate,
            validator: (value) {
              if (value == null || value.isEmpty || ipv4RegExp.hasMatch(value))
                return null;
              return l10n.configureBasicBindBadTcpAddr;
            },
            onSaved: (value) => config.bind = value,
          ),
          // flush
          CardSettingsInt(
            label: l10n.configureBasicFlushIntervalLabel,
            unitLabel: l10n.configureBasicFlushIntervalUnitLabel,
            initialValue: config.flushIntervalMs,
            autovalidate: autoValidate,
            validator: (value) {
              if (value != null) {
                if (value < 0) return l10n.configureBasicFlushIntervalNegtive;
                // MaxUint32
                if (value > 1 << 32 - 1)
                  return l10n.configureBasicFlushIntervalUint32;
              }
              return null;
            },
            onSaved: (value) => config.flushIntervalMs = value,
          ),
          // token
          CardSettingsParagraph(
            label: l10n.configureBasicTokenLabel,
            initialValue: config.token,
            numberOfLines: _tokenLines,
            onSaved: (value) => config.token = value,
          ),
          Container(height: 4.0)
        ],
      );
    }

    return ConfigureLeafPage(
      title: Text(l10n.configureBasicTitle),
      onPortraitLayout: _buildPortraitLayout,
      onLandscapeLayout: _buildPortraitLayout,
      onSaved: () => Navigator.of(context).pop(),
    );
  }
}
