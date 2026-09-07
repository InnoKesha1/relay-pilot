import 'dart:io';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter/foundation.dart';
import 'package:file_picker/file_picker.dart';
import 'package:hiddify/features/relay/relay_qr.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:hiddify/core/router/dialog/dialog_notifier.dart';
import 'package:hiddify/features/profile/notifier/profile_notifier.dart';
import 'package:hiddify/features/relay/relay_profile.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

class AddProfileModal extends HookConsumerWidget {
  const AddProfileModal({super.key, this.url});
  final String? url;
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final text = useTextEditingController(text: url ?? '');
    final error = useState<String?>(null);
    final status = ref.watch(addProfileNotifierProvider);
    ref.listen(addProfileNotifierProvider, (_, next) {
      if (next is AsyncData && next.value != null && context.mounted) Navigator.of(context).pop();
    });
    Future<void> submit() async {
      try {
        RelayProfile.subscriptionUri(text.text);
      } catch (_) {
        error.value = 'Вставь персональную HTTPS-ссылку, полученную от владельца сервиса.';
        return;
      }
      error.value = null;
      await ref.read(addProfileNotifierProvider.notifier).addClipboard(text.text.trim());
    }

    return SafeArea(
      child: Padding(
        padding: EdgeInsets.fromLTRB(24, 24, 24, 24 + MediaQuery.viewInsetsOf(context).bottom),
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Row(
                children: [
                  Expanded(child: Text('Добавить доступ', style: Theme.of(context).textTheme.headlineSmall)),
                  IconButton(
                    onPressed: status.isLoading ? null : () => Navigator.of(context).pop(),
                    icon: const Icon(Icons.close),
                  ),
                ],
              ),
              const SizedBox(height: 16),
              const Text('Персональная ссылка открывает доступ к сервису. Храни её как пароль.'),
              const SizedBox(height: 20),
              TextField(
                controller: text,
                obscureText: true,
                autocorrect: false,
                enableSuggestions: false,
                enabled: !status.isLoading,
                decoration: InputDecoration(
                  labelText: 'Ссылка доступа',
                  errorText: error.value,
                  border: const OutlineInputBorder(),
                ),
                onSubmitted: (_) => submit(),
              ),
              const SizedBox(height: 12),
              Wrap(
                spacing: 8,
                children: [
                  TextButton.icon(
                    onPressed: status.isLoading
                        ? null
                        : () async {
                            try {
                              final pick = await FilePicker.platform.pickFiles(
                                type: FileType.custom,
                                allowedExtensions: ['png', 'jpg', 'jpeg', 'webp'],
                              );
                              if (pick == null) return;
                              final file = File(pick.files.single.path!);
                              if (await file.length() > 8 * 1024 * 1024) throw const FormatException();
                              final value = await compute(decodeRelayQr, await file.readAsBytes());
                              if (context.mounted) {
                                text.text = value;
                                error.value = null;
                              }
                            } catch (_) {
                              if (context.mounted) error.value = 'Не удалось прочитать QR-код Relay Pilot.';
                            }
                          },
                    icon: const Icon(Icons.image_outlined),
                    label: const Text('QR из файла'),
                  ),
                  TextButton.icon(
                    onPressed: status.isLoading
                        ? null
                        : () async {
                            final data = await Clipboard.getData(Clipboard.kTextPlain);
                            text.text = data?.text ?? '';
                          },
                    icon: const Icon(Icons.content_paste),
                    label: const Text('Вставить'),
                  ),
                  if (Platform.isAndroid)
                    TextButton.icon(
                      onPressed: status.isLoading
                          ? null
                          : () async {
                              final value = await ref.read(dialogNotifierProvider.notifier).showQrScanner();
                              if (value != null) text.text = value;
                            },
                      icon: const Icon(Icons.qr_code_scanner),
                      label: const Text('Сканировать QR'),
                    ),
                ],
              ),
              const SizedBox(height: 12),
              FilledButton(
                onPressed: status.isLoading ? null : submit,
                child: Text(status.isLoading ? 'Проверяем профиль…' : 'Добавить'),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
