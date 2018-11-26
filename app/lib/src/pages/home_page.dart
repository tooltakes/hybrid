import 'dart:async';

import 'package:flutter/material.dart';

import '../const/routes.dart';
import '../l10n/app.l10n.dart';
import '../widgets/drawer.dart';
import '../widgets/choice.dart';

class HomePage extends StatefulWidget {
  HomePage({Key key, this.title, this.choices}) : super(key: key);

  final String title;
  final List<Choice> choices;

  @override
  _HomePageState createState() => _HomePageState();
}

class _HomePageState extends State<HomePage> {
  StreamSubscription<HybridState> _listener;
  String _port = "Unknown";

  @override
  void initState() {
    super.initState();
  }

  @override
  void dispose() {
    _listener.cancel();
    _listener = null;
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context);
    return Scaffold(
      drawer: AppDrawer(),
      appBar: AppBar(
        title: Text(widget.title),
        actions: <Widget>[AppPopupMenuButton(choices: widget.choices)],
      ),
      body: _SimpleHybrid(
        status: _status,
        onChanged: _waiting ? null : _goingToChange,
      ),
      floatingActionButton: FloatingActionButton(
        onPressed: _status.on ? _showUsage : null,
        tooltip: 'Usage',
        child: Icon(Icons.wifi_tethering),
      ), // This trailing comma makes auto-formatting nicer for build methods.
    );
  }
}

class _SimpleHybrid extends StatelessWidget {
  _HybridStatus status;
  ValueChanged<bool> onChanged;

  _SimpleHybrid({this.status, this.onChanged});

  @override
  Widget build(BuildContext context) {
    return Center(
      // Center is a layout widget. It takes a single child and positions it
      // in the middle of the parent.
      child: Column(
        // Column is also layout widget. It takes a list of children and
        // arranges them vertically. By default, it sizes itself to fit its
        // children horizontally, and tries to be as tall as its parent.
        //
        // Invoke "debug paint" (press "p" in the console where you ran
        // "flutter run", or select "Toggle Debug Paint" from the Flutter tool
        // window in IntelliJ) to see the wireframe for each widget.
        //
        // Column has various properties to control how it sizes itself and
        // how it positions its children. Here we use mainAxisAlignment to
        // center the children vertically; the main axis here is the vertical
        // axis because Columns are vertical (the cross axis would be
        // horizontal).
        mainAxisAlignment: MainAxisAlignment.center,
        children: <Widget>[
          SizedBox.fromSize(
            child: Switch(value: status.on, onChanged: onChanged),
            size: Size(64.0, 64.0),
          ),
          Text(
            status.text,
            style: Theme.of(context).textTheme.display1,
          ),
          Text(
            status.err,
          ),
        ],
      ),
    );
  }
}

class _HybridStatus {
  final bool on;
  final String text;
  final String err;

  const _HybridStatus({this.on, this.text, this.err});
}
