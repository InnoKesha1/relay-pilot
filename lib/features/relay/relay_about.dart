import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:url_launcher/url_launcher.dart';

Future<void> showRelayAbout(BuildContext context) => showDialog<void>(
  context: context,
  builder: (context) => AlertDialog(
    title: const Text('Relay Pilot'),
    content: const SingleChildScrollView(
      child: Text(
        'Закрытый некоммерческий пилот.\n\nОснован на Hiddify App 4.1.1, Hiddify Core и sing-box. Изменения: оформление, две цепочки, проверка профилей и собственный сервис доступа.\n\nЛицензия: Hiddify Extended GPL v3.\nАвторы исходного приложения: Hiddify и участники проекта.',
      ),
    ),
    actions: [
      TextButton(
        onPressed: () => launchUrl(Uri.parse('https://github.com/InnoKesha1/relay-pilot/releases')),
        child: const Text('Обновления'),
      ),
      TextButton(
        onPressed: () => launchUrl(Uri.parse('https://github.com/hiddify/hiddify-app')),
        child: const Text('Hiddify'),
      ),
      TextButton(
        onPressed: () => launchUrl(Uri.parse('https://github.com/InnoKesha1/relay-pilot')),
        child: const Text('Исходники'),
      ),
      TextButton(
        onPressed: () async {
          final license = await rootBundle.loadString('LICENSE.md');
          if (context.mounted)
            await showDialog<void>(
              context: context,
              builder: (context) => AlertDialog(
                title: const Text('Лицензия'),
                content: SingleChildScrollView(child: SelectableText(license)),
                actions: [TextButton(onPressed: () => Navigator.pop(context), child: const Text('Закрыть'))],
              ),
            );
        },
        child: const Text('Лицензия'),
      ),
      TextButton(onPressed: () => Navigator.pop(context), child: const Text('Закрыть')),
    ],
  ),
);
