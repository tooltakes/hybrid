import 'package:flutter_local_notifications/flutter_local_notifications.dart';

import './app_env.dart';

class LoadAppEnv extends AppEnv {
  @override
  final bool dev = true;

  @override
  final String channelId = 'io.github.empirefox.hybrid';
  @override
  final Importance importance = Importance.High;
  @override
  final int notificationId = 1;
  @override
  final String androidIcon = 'app_icon';

  @override
  String hybridDirName = '.hybrid';
}
