import 'dart:ui';
import 'package:flutter/widgets.dart';
import 'package:flutter_local_notifications/flutter_local_notifications.dart';
import 'package:flutter_dial_go/flutter_dial_go.dart';
import 'package:grpc/grpc.dart';

import '../l10n/app.l10n.dart';

class AppGo {
  static bool _goInited = false;
  static String _channelId;
  static Importance _importance;
  static int _notificationId;

  static FlutterLocalNotificationsPlugin _notificationsPlugin;
  static NotificationDetails _platformChannelSpecifics;

  static Future<void> initOnce(
      {@required String channelId,
      @required Importance importance,
      @required int notificationId,
      @required String androidIcon,
      @required SelectNotificationCallback onSelectNotification}) async {
    if (_goInited) return;
    _goInited = true;

    _channelId = channelId;
    _importance = importance;

    // init flutter_local_notifications
    final initializationSettings = InitializationSettings(
      AndroidInitializationSettings(androidIcon),
      IOSInitializationSettings(),
    );
    _notificationsPlugin = FlutterLocalNotificationsPlugin();
    await _notificationsPlugin.initialize(initializationSettings,
        onSelectNotification: onSelectNotification);

    // use default string only first time
    final l10n = await AppLocalizations.load(window.locale);

    // init flutter_dial_go
    await Conn.notificationChannel(
      channelId: channelId,
      importance: importance.value,
      name: l10n.notificationChannelName,
      description: l10n.notificationChannelDesc,
    );
    await Conn.startGoWithService(
      channelId: channelId,
      notificationId: _notificationId,
      title: l10n.serviceTitle,
      text: l10n.nodeStopped,
    );

    await _setupNotificationLocal(l10n);
    AppLocalizations.addOnLoad(_setupNotificationLocal);
  }

  static ClientChannel grpcDial(int port) {
    return ClientChannel(
      'go',
      port: port,
      options: ChannelOptions(
        credentials: ChannelCredentials.insecure(),
        connect: _connect,
      ),
    );
  }

  static Future<void> showNodeStatus(BuildContext context,
      {String status, String payload}) async {
    final l10n = AppLocalizations.of(context);
    await _notificationsPlugin.show(
      _notificationId,
      l10n.serviceTitle,
      status,
      _platformChannelSpecifics,
      payload: payload,
    );
  }

  static Future<void> destroy() async {
    AppLocalizations.removeOnLoad(_setupNotificationLocal);
    await Conn.stopGo();
  }

  static Future<void> _setupNotificationLocal(AppLocalizations l10n) async {
    _platformChannelSpecifics = NotificationDetails(
        AndroidNotificationDetails(
          _channelId,
          l10n.notificationChannelName,
          l10n.notificationChannelDesc,
          importance: _importance,
          priority: Priority.High,
          playSound: false,
          ongoing: true,
          autoCancel: false,
        ),
        IOSNotificationDetails());
  }

  static Future<Http2Streams> _connect(
      String host, int port, ChannelCredentials credentials) async {
    // ignore: close_sinks
    final conn = await Conn.dial(port);
    return Http2Streams(conn.receiveStream, conn);
  }
}
