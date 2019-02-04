import 'package:bloc/bloc.dart';

import '../global.dart';
import '../protos.dart';

class HybridConfigState {
  static final HybridConfigState getting = HybridConfigState._(null, null);
  static final HybridConfigState saving = HybridConfigState._(null, null);

  final Config config;
  final String error;

  HybridConfigState.saved(this.config) : error = null;
  HybridConfigState.failed(this.error) : config = null;
  HybridConfigState._(this.config, this.error);
}

abstract class HybridConfigEvent {}

class HybirdConfigGetEvent extends HybridConfigEvent {
  final bool load;

  HybirdConfigGetEvent(this.load);

  @override
  String toString() => 'Get config';
}

class HybirdConfigSaveEvent extends HybridConfigEvent {
  final Config config;

  HybirdConfigSaveEvent(this.config);

  @override
  String toString() => 'Save config';
}

class HybridConfigBloc extends Bloc<HybridConfigEvent, HybridConfigState> {
  @override
  HybridConfigState get initialState => null;

  @override
  Stream<HybridConfigState> mapEventToState(
      HybridConfigState state, HybridConfigEvent event) async* {
    if (event is HybirdConfigGetEvent) {
      yield HybridConfigState.getting;
      try {
        final config = await AppHybrid.getConfig(load: event.load);
        yield HybridConfigState.saved(config);
      } catch (e) {
        yield HybridConfigState.failed(e.toString());
      }
    }

    if (event is HybirdConfigSaveEvent) {
      yield HybridConfigState.saving;
      try {
        await AppHybrid.saveConfig(event.config);
        yield HybridConfigState.saved(event.config.clone());
      } catch (e) {
        yield HybridConfigState.failed(e.toString());
      }
    }
  }
}
