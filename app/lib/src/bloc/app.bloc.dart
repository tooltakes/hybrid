import 'package:bloc/bloc.dart';

abstract class AppEvent {}

class Start extends AppEvent {
  @override
  String toString() => 'Start';
}

class Stop extends AppEvent {
  @override
  String toString() => 'Stop';
}

class AppBloc extends Bloc<AppEvent, int> {
  @override
  int get initialState => 0;

  @override
  Stream<int> mapEventToState(int state, AppEvent event) async* {
    if (event is Start) {
      yield state + 1;
    }
    if (event is Stop) {
      yield state - 1;
    }
  }
}
