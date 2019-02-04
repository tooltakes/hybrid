import 'dart:async';
import 'package:flutter/material.dart';
import 'package:card_settings/card_settings.dart';

import '../../l10n/app.l10n.dart';

typedef CardSettings CardSettingsLayoutCallback(bool autoValidate);

class ConfigureLeafPage extends StatefulWidget {
  final Widget title;
  final CardSettingsLayoutCallback onPortraitLayout;
  final CardSettingsLayoutCallback onLandscapeLayout;
  final VoidCallback onSaved;

  const ConfigureLeafPage({
    Key key,
    @required this.title,
    @required this.onPortraitLayout,
    @required this.onLandscapeLayout,
    @required this.onSaved,
  }) : super(key: key);

  @override
  _ConfigureLeafPageState createState() => _ConfigureLeafPageState();
}

class _ConfigureLeafPageState extends State<ConfigureLeafPage> {
  final GlobalKey<FormState> _formKey = GlobalKey<FormState>();
  final GlobalKey<ScaffoldState> _scaffoldKey = GlobalKey<ScaffoldState>();
  bool _autoValidate = false;

  @override
  Widget build(BuildContext context) {
    final orientation = MediaQuery.of(context).orientation;

    return Scaffold(
      key: _scaffoldKey,
      backgroundColor: Theme.of(context).backgroundColor,
      appBar: appBar(),
      body: Form(
        key: _formKey,
        child: (orientation == Orientation.portrait)
            ? widget.onPortraitLayout(_autoValidate)
            : widget.onLandscapeLayout(_autoValidate),
      ),
    );
  }

  AppBar appBar() => AppBar(
        title: widget.title,
        actions: <Widget>[
          IconButton(
            icon: Icon(Icons.check),
            onPressed: _savePressed,
          ),
        ],
        leading: IconButton(
          icon: Icon(Icons.arrow_back),
          onPressed: _backPressed,
        ),
      );

  void _savePressed() {
    final form = _formKey.currentState;

    // TODO add server validation?
    if (form.validate()) {
      form.save();
      widget.onSaved();
    } else {
      setState(() => _autoValidate = true);
    }
  }

  Future<void> _backPressed() async {
    final l10n = AppLocalizations.of(context);

    return showDialog<void>(
      context: context,
      barrierDismissible: true,
      builder: (BuildContext context) {
        return AlertDialog(
          title: Text(l10n.configureBackAlertTitle),
          content: SingleChildScrollView(
            child: ListBody(
              children: <Widget>[
                Text(l10n.configureBackAlertContent),
              ],
            ),
          ),
          actions: <Widget>[
            FlatButton(
              child: Text(l10n.configureBackAlertStay),
              onPressed: () => Navigator.of(context).pop(),
            ),
            FlatButton(
              child: Text(l10n.configureBackAlertGoBack),
              onPressed: _goBack,
            ),
          ],
        );
      },
    );
  }

  void _goBack() => Navigator.of(context).pop();
}
