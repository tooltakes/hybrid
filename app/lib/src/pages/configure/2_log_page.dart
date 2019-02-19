import 'package:flutter/material.dart';
import 'package:card_settings/card_settings.dart';

import '../../l10n/app.l10n.dart';
import '../../protos.dart';

import './configure_leaf_page.dart';

class ConfigLogPage extends StatelessWidget {
  final Log configLog;

  const ConfigLogPage({Key key, this.configLog}) : super(key: key);

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context);

    CardSettings _buildPortraitLayout(bool autoValidate) {
      return CardSettings(
        children: <Widget>[
          // dev
          CardSettingsSwitch(
            label: l10n.devModeLabel,
            initialValue: configLog.dev,
            onSaved: (value) => configLog.dev = value,
          ),
          // level
          CardSettingsListPicker(
            label: l10n.configureLogLevelLabel,
            initialValue: configLog.level,
            autovalidate: autoValidate,
            options: <String>[
              'debug',
              'info',
              'warn',
              'error',
              'dpanic',
              'panic',
              'fatal',
            ],
            validator: (String value) {
              return value == null || value.isEmpty
                  ? l10n.configureLogLevelEmpty
                  : null;
            },
            onSaved: (value) => configLog.level = value,
          ),
          // target
          CardSettingsText(
            label: l10n.configureLogTargetLabel,
            hintText: l10n.configureLogTargetHint,
            initialValue: configLog.target,
            autovalidate: autoValidate,
            validator: (value) {
              // sync with golang
              return null;
            },
            onSaved: (value) => configLog.target = value,
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
