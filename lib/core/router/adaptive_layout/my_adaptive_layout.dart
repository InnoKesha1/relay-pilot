import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

/// Service access and route selection are exposed by the dedicated home screen.
class MyAdaptiveLayout extends StatelessWidget {
  const MyAdaptiveLayout({
    super.key,
    required this.navigationShell,
    required this.isMobileBreakpoint,
    required this.showProfilesAction,
  });
  final StatefulNavigationShell navigationShell;
  final bool isMobileBreakpoint;
  final bool showProfilesAction;
  @override
  Widget build(BuildContext context) => navigationShell;
}
