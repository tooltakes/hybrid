import 'package:flutter_local_notifications/flutter_local_notifications.dart';

abstract class AppEnv {
  AppEnv() {
    assert(dev != null);
    assert(channelId != null);
    assert(notificationId != null);
    assert(importance != null);
    assert(androidIcon != null);
    assert(_appEnv == null);
    _appEnv = this;
  }

  /// dev mode if true
  bool get dev;

  String get channelId;
  int get notificationId;
  Importance get importance;
  String get androidIcon;

  String get hybridDirName;
}

AppEnv _appEnv;
AppEnv get appEnv => _appEnv;
