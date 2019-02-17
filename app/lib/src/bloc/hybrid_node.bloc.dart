import 'package:bloc/bloc.dart';

import '../global.dart';

class HybridNodeState {
  static final HybridNodeState waiting = HybridNodeState._(null, null);
  static final HybridNodeState running = HybridNodeState._(true, null);
  static final HybridNodeState stopped = HybridNodeState._(false, null);

  /// null means unknown state, aka waiting
  final bool isRunning;
  final String error;

  HybridNodeState.error(this.error) : isRunning = false;
  HybridNodeState._(this.isRunning, this.error);
}

enum HybridNodeEvent { start, stop }

class HybridNodeBloc extends Bloc<HybridNodeEvent, HybridNodeState> {
  @override
  HybridNodeState get initialState => HybridNodeState.stopped;

  @override
  Stream<HybridNodeState> mapEventToState(
      HybridNodeState state, HybridNodeEvent event) {
    switch (event) {
      case HybridNodeEvent.start:
        return _handleStart(state, event);
      case HybridNodeEvent.stop:
        return _handleStop(state, event);
    }
    throw ArgumentError.value(state, 'HybridNodeBloc', 'unknown state');
  }

  Stream<HybridNodeState> _handleStart(
      HybridNodeState state, HybridNodeEvent event) async* {
    if (state == HybridNodeState.running || state == HybridNodeState.waiting)
      return;

    yield HybridNodeState.waiting;
    try {
      await AppHybrid.start();
      yield HybridNodeState.running;
    } catch (e) {
      yield HybridNodeState.error(e.toString());
    }
  }

  Stream<HybridNodeState> _handleStop(
      HybridNodeState state, HybridNodeEvent event) async* {
    if (state == HybridNodeState.stopped || state == HybridNodeState.waiting)
      return;

    yield HybridNodeState.waiting;
    try {
      await AppHybrid.stop();
    } catch (e) {}
    yield HybridNodeState.stopped;
  }
}
