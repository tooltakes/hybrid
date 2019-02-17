import 'package:flutter/material.dart';
import 'package:card_settings/card_settings.dart';

import '../../l10n/app.l10n.dart';
import '../../protos.dart';

import './configure_leaf_page.dart';

class ConfigLogPage extends StatelessWidget {
  static final RegExp ipv4RegExp = RegExp(
      r"^(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$");

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
            label: l10n.configureLogDevLabel,
            initialValue: configLog.dev,
            onSaved: (value) => configLog.dev = value,
          ),
          // level
          CardSettingsListPicker(
            label: 'Type',
            initialValue: _ponyModel.type,
            hintText: 'Select One',
            autovalidate: _autoValidate,
            options: <String>['Earth', 'Unicorn', 'Pegasi', 'Alicorn'],
            validator: (String value) {
              if (value == null || value.isEmpty)
                return 'You must pick a type.';
              return null;
            },
            onSaved: (value) => _ponyModel.type = value,
            onChanged: (value) => _showSnackBar('Type', value),
          ),
          // target
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
