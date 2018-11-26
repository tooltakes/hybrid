import 'package:flutter/material.dart';

// show Divider if Choice is null
class Choice {
  const Choice({this.title, this.icon, this.route, this.onSelected});

  final String title;
  final IconData icon;

  // onSelected > route
  final String route;
  // onSelected > route
  final PopupMenuItemSelected<Choice> onSelected;
}

class AppPopupMenuButton extends StatelessWidget {
  final List<Choice> choices;

  const AppPopupMenuButton({Key key, this.choices}) : super(key: key);

  @override
  Widget build(BuildContext context) {
    return PopupMenuButton<Choice>(
      onSelected: (choice) {
        if (choice.onSelected != null) {
          choice.onSelected(choice);
          return;
        }
        Navigator.pushNamed(context, choice.route);
      },
      itemBuilder: (context) {
        return choices.map((choice) {
          if (choice == null) {
            return PopupMenuDivider();
          }
          return PopupMenuItem<Choice>(
            value: choice,
            child: Text(choice.title),
          );
        }).toList();
      },
    );
  }
}
