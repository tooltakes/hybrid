import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:bloc/bloc.dart';
import 'package:flutter_bloc/flutter_bloc.dart';

import './bloc/hybrid_config.bloc.dart';
import './const/routes.dart';
import './env.dart';
import './global/app_go.dart';
import './l10n/app.l10n.dart';
import './protos.dart';

import './pages/about_page.dart';
import './pages/configure/1_basic_page.dart';
import './pages/home_page.dart';

class SimpleBlocDelegate extends BlocDelegate {
  @override
  void onTransition(Transition transition) {
    print(transition.toString());
  }
}

class MyApp extends StatefulWidget {
  @override
  State<StatefulWidget> createState() => MyAppState();
}

class MyAppState extends State<MyApp> {
  final _hybridConfigBloc = HybridConfigBloc();

  @override
  Widget build(BuildContext context) {
    AppGo.initOnce(
      channelId: appEnv.channelId,
      importance: appEnv.importance,
      notificationId: appEnv.notificationId,
      androidIcon: appEnv.androidIcon,
      onSelectNotification: onSelectNotification,
    );
    return MaterialApp(
      onGenerateTitle: (BuildContext context) =>
          AppLocalizations.of(context).appName,
      localizationsDelegates: [
        const AppLocalizationsDelegate(),
        GlobalMaterialLocalizations.delegate,
        GlobalWidgetsLocalizations.delegate,
      ],
      supportedLocales: [
        const Locale('en', ''),
      ],
      theme: ThemeData(
        // This is the theme of your application.
        //
        // Try running your application with "flutter run". You'll see the
        // application has a blue toolbar. Then, without quitting the app, try
        // changing the primarySwatch below to Colors.green and then invoke
        // "hot reload" (press "r" in the console where you ran "flutter run",
        // or simply save your changes to "hot reload" in a Flutter IDE).
        // Notice that the counter didn't reset back to zero; the application
        // is not restarted.
        primarySwatch: Colors.blue,
      ),
      initialRoute: AppRoutes.home,
      routes: {
        AppRoutes.home: (context) => BlocProvider<HybridConfigBloc>(
              bloc: _hybridConfigBloc,
              child: HomePage(
                title: 'Home',
              ),
            ),
        AppRoutes.configure: (context) => BlocProvider<HybridConfigBloc>(
              bloc: _hybridConfigBloc,
              child: ConfigBasicPage(
                config: Config(),
              ),
            ),
        AppRoutes.about: (context) => AboutPage(),
      },
    );
  }

  Future<Null> onSelectNotification(String payload) async {
    if (payload != null) {
      debugPrint('notification payload: ' + payload);
    }
    await Navigator.pushNamed(context, AppRoutes.home);
  }
}
