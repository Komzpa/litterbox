// ignore: unused_import
import 'package:intl/intl.dart' as intl;

import 'app_localizations.dart';

// ignore_for_file: type=lint

/// The translations for Russian (`ru`).
class AppLocalizationsRu extends AppLocalizations {
  AppLocalizationsRu([String locale = 'ru']) : super(locale);

  @override
  String get nowSection => 'Сейчас';

  @override
  String get laterSection => 'Позже';

  @override
  String missedCount(int count) {
    return 'Пропущено ($count)';
  }

  @override
  String get emptyCards => 'Нет карточек';

  @override
  String get retry => 'Повторить';

  @override
  String get oldServer => 'Сервер старше приложения';

  @override
  String get restart => 'Перезапустить';

  @override
  String get done => 'Готово';

  @override
  String get note => 'Заметка';

  @override
  String get noteHint => 'Что нужно сообщить генератору задач?';

  @override
  String get cancel => 'Отмена';

  @override
  String get save => 'Сохранить';

  @override
  String get doneWithNote => 'Готово с заметкой';

  @override
  String get loadError => 'Не удалось загрузить карточки';

  @override
  String actionError(Object error) {
    return 'Не удалось выполнить действие: $error';
  }

  @override
  String noteSaveError(Object error) {
    return 'Не удалось сохранить заметку: $error';
  }

  @override
  String outdatedApi(int client, int server) {
    return 'Приложение устарело (API $client), серверу нужен $server';
  }

  @override
  String aboutBuild(Object clientBuild, Object serverBuild) {
    return 'О приложении · приложение $clientBuild, сервер $serverBuild';
  }

  @override
  String get createReminder => 'Добавить напоминание';

  @override
  String get reminderTitle => 'Название';

  @override
  String get reminderWhen => 'Дата и время';

  @override
  String get create => 'Создать';
}
