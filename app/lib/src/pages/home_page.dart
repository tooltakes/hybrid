import 'package:flutter/material.dart';

import '../widgets/choice.dart';
import '../widgets/drawer.dart';

class HomePage extends StatefulWidget {
  HomePage({Key key, this.title, this.choices}) : super(key: key);

  final String title;
  final List<Choice> choices;

  @override
  _HomePageState createState() => _HomePageState();
}

class _HomePageState extends State<HomePage> {
  @override
  void initState() {
    super.initState();
  }

  @override
  void dispose() {
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      drawer: AppDrawer(),
      appBar: AppBar(
        title: Text(widget.title),
        actions: <Widget>[AppPopupMenuButton(choices: widget.choices)],
      ),
      body: Text('Hello world'),
      floatingActionButton: FloatingActionButton(
        onPressed: null,
        tooltip: 'Usage',
        child: Icon(Icons.wifi_tethering),
      ),
    );
  }
}
