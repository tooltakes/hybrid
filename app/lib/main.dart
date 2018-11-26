import 'package:flutter/material.dart';
import 'package:bloc/bloc.dart';

import './src/app.dart';

// default env is dev
import './src/env/env.dev.dart';

class SimpleBlocDelegate extends BlocDelegate {
  @override
  void onTransition(Transition transition) {
    print(transition.toString());
  }
}

void main() {
  LoadAppEnv();
  BlocSupervisor().delegate = SimpleBlocDelegate();
  runApp(MyApp());
}
