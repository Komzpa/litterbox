// ignore: unused_import
import 'package:intl/intl.dart' as intl;

import 'app_localizations.dart';

// ignore_for_file: type=lint

/// The translations for Belarusian (`be`).
class AppLocalizationsBe extends AppLocalizations {
  AppLocalizationsBe([String locale = 'be']) : super(locale);

  @override
  String get nowSection => 'Зараз';

  @override
  String get laterSection => 'Пазней';

  @override
  String missedCount(int count) {
    return 'Прапушчаныя ($count)';
  }

  @override
  String get emptyCards => 'Няма картак';

  @override
  String get retry => 'Паўтарыць';

  @override
  String get oldServer => 'Сервер старэйшы за праграму';

  @override
  String get restart => 'Перазапусціць';

  @override
  String get done => 'Гатова';

  @override
  String get note => 'Нататка';

  @override
  String get noteHint => 'Што трэба сказаць генератару задач?';

  @override
  String get cancel => 'Адмена';

  @override
  String get save => 'Захаваць';

  @override
  String get doneWithNote => 'Гатова з нататкай';

  @override
  String get loadError => 'Не атрымалася загрузіць карткі';

  @override
  String actionError(Object error) {
    return 'Не атрымалася выканаць дзеянне: $error';
  }

  @override
  String noteSaveError(Object error) {
    return 'Не атрымалася захаваць нататку: $error';
  }

  @override
  String outdatedApi(int client, int server) {
    return 'Праграма састарэла (API $client), серверу патрэбны $server';
  }

  @override
  String aboutBuild(Object clientBuild, Object serverBuild) {
    return 'Аб праграме · праграма $clientBuild, сервер $serverBuild';
  }

  @override
  String get createReminder => 'Дадаць напамін';

  @override
  String get reminderTitle => 'Назва';

  @override
  String get reminderWhen => 'Дата і час';

  @override
  String get create => 'Стварыць';

  @override
  String get pinned => 'Замацаваныя';

  @override
  String get archiveBundle => 'Архіваваць групу';

  @override
  String get pin => 'Замацаваць';

  @override
  String get unpin => 'Адмацаваць';

  @override
  String get takeOutOfBundle => 'Прыбраць з групы';

  @override
  String get snooze => 'Адкласці';

  @override
  String get laterToday => 'Пазней сёння';

  @override
  String get tomorrowMorning => 'Заўтра раніцай';

  @override
  String get nextWeek => 'На наступным тыдні';

  @override
  String get customDateTime => 'Выбраць дату і час';

  @override
  String bundleCount(Object bundle, int count) {
    return '$bundle ($count)';
  }

  @override
  String get journal => 'Журнал';

  @override
  String get journalEntryHint => 'Новы запіс';

  @override
  String get mcpSettings => 'Токены MCP';

  @override
  String get createToken => 'Стварыць токен';

  @override
  String get revokeToken => 'Адклікаць';

  @override
  String get tokenShownOnce =>
      'Скапіруйце токен зараз. Пазней ён не будзе паказаны.';
}
