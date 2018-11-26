import 'package:grpc/grpc.dart';
import 'package:bloc/bloc.dart';
import 'package:path_provider/path_provider.dart';

import '../protos.dart';
import '../global/app_go.dart';
import '../env.dart';

const hybridPort = 1;

// State

abstract class HybridState {}

// Hybrid Node State

class HybridNodeState extends HybridState {
  static final HybridNodeState waiting = HybridNodeState._(null, null);
  static final HybridNodeState running = HybridNodeState._(true, null);
  static final HybridNodeState stopped = HybridNodeState._(false, null);

  /// null means unknown state, aka waiting
  final bool isRunning;
  final String error;

  HybridNodeState.error(this.error) : isRunning = false;
  HybridNodeState._(this.isRunning, this.error);
}

// Event

abstract class HybridEvent {}

// Hybrid Node Event

abstract class HybridNodeEvent {}

class Start extends HybridNodeEvent {
  @override
  String toString() => 'Start';
}

class Decrement extends HybridNodeEvent {
  @override
  String toString() => 'Decrement';
}

class HybridBloc extends Bloc<HybridEvent, HybridState> {
  @override
  HybridState get initialState => HybridNodeState.stopped;

  static HybridClient _hybrid;
  static ClientChannel _hybridChannel;

  static String _localPath;
  static StartRequest _defaultStartRequest;

  static HybridClient get hybrid {
    if (_hybrid == null) {
      _hybridChannel = AppGo.grpcDial(hybridPort);
      _hybrid = HybridClient(_hybridChannel);
    }
    return _hybrid;
  }

  static Future<StartRequest> get defaultStartRequest async {
    if (_defaultStartRequest != null) return _defaultStartRequest;
    final appDocPath = await localPath;
    if (_defaultStartRequest != null) return _defaultStartRequest;
    _defaultStartRequest = StartRequest()
      ..root = '${appDocPath}/${appEnv.hybridDirName}';
    return _defaultStartRequest;
  }

  static Future<String> get localPath async {
    if (_localPath != null) return _localPath;
    final directory = await getApplicationDocumentsDirectory();
    if (_localPath != null) return _localPath;
    return _localPath = directory.path;
  }

  @override
  Stream<HybridState> mapEventToState(HybridState state, HybridEvent event) {
    return _mapEventToState(state, event).skipWhile((s) => s == null);
  }

  Stream<HybridState> _mapEventToState(HybridState state, HybridEvent event) {
    if (event is Start) {
      return handleEventStart(state, event);
    } else if (event is Decrement) {}
  }

  Stream<HybridState> handleEventStart(
      HybridState state, HybridEvent event) async* {
    if (state == HybridNodeState.running || state == HybridNodeState.waiting)
      return;

    yield HybridNodeState.waiting;
    try {
      await hybrid.start(await defaultStartRequest);
      yield HybridNodeState.running;
    } catch (e) {
      yield HybridNodeState.error(e.toString());
    }
  }

  @override
  void dispose() {
    super.dispose();
    _hybridChannel.terminate();
  }
}
