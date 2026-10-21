#include <QDir>
#include <QDirIterator>
#include <QFile>
#include <QRegularExpression>
#include <QStringList>
#include <QtTest>

// i18n guard: fails when a user-visible QML literal is left unwrapped,
// or when the ru catalogs are missing/stale. Covers R30 system-locale display.
class I18nTest : public QObject {
    Q_OBJECT
private slots:
    void unwrappedLiterals()
    {
        const QString qtDir = QStringLiteral(LB_QT_SOURCE_DIR);
        const QStringList roots = {
            qtDir + QStringLiteral("/qml"),
            qtDir + QStringLiteral("/pages"),
            qtDir + QStringLiteral("/src/qml"),
        };
        // text/title/placeholder/label/tooltip/accessible bound to a raw
        // "Literal" instead of qsTr(...). Brand and technical values exempt.
        const QRegularExpression bound(
            QStringLiteral(R"((?:^|[{\s;])(?:text|title|placeholderText|Accessible\.name|ToolTip\.text|label)\s*:\s*\"([^\"]+)\")"));
        const QStringList allowExact = {
            QStringLiteral("Litterbox"), QStringLiteral("monospace"),
            QStringLiteral("linux"), QStringLiteral("android"),
            QStringLiteral("••••••••••••"),
        };
        QStringList offenders;
        for (const QString &root : roots) {
            QDirIterator it(root, {QStringLiteral("*.qml")},
                            QDir::Files, QDirIterator::Subdirectories);
            while (it.hasNext()) {
                const QString path = it.next();
                QFile f(path);
                if (!f.open(QIODevice::ReadOnly | QIODevice::Text))
                    continue;
                const QString body = QString::fromUtf8(f.readAll());
                auto lines = body.split(QLatin1Char('\n'));
                for (int i = 0; i < lines.size(); ++i) {
                    const QString line = lines.at(i).trimmed();
                    if (line.startsWith(QStringLiteral("//")))
                        continue;
                    auto m = bound.match(lines.at(i));
                    if (!m.hasMatch())
                        continue;
                    const QString lit = m.captured(1);
                    if (lit.isEmpty() || allowExact.contains(lit))
                        continue;
                    // Bindings, URLs, single words that are ids, or pure
                    // punctuation/numbers are not user prose.
                    if (lit.contains(QLatin1Char('{')) || lit.contains(QLatin1Char('}'))
                        || lit.startsWith(QStringLiteral("http"))
                        || lit.startsWith(QStringLiteral(":")))
                        continue;
                    offenders << QStringLiteral("%1:%2: %3").arg(path, QString::number(i + 1), lit);
                }
            }
        }
        if (!offenders.isEmpty())
            qWarning() << "unwrapped literals:\n" + offenders.join(QLatin1Char('\n'));
        QVERIFY2(offenders.isEmpty(), "user-visible QML literals must use qsTr()");
    }

    void russianCatalogComplete()
    {
        const QString qtDir = QStringLiteral(LB_QT_SOURCE_DIR);
        const QString screens = qtDir + QStringLiteral("/i18n/screens_ru.ts");
        const QString actions = qtDir + QStringLiteral("/src/qml/actions_ru.ts");
        for (const QString &path : {screens, actions}) {
            QFile f(path);
            QVERIFY2(f.open(QIODevice::ReadOnly | QIODevice::Text),
                     qPrintable(QStringLiteral("missing %1").arg(path)));
            const QString body = QString::fromUtf8(f.readAll());
            QVERIFY2(!body.contains(QStringLiteral("type=\"unfinished\"")),
                     qPrintable(QStringLiteral("%1 has unfinished entries").arg(path)));
        }
        // Key screens must actually be translated (not English passthrough).
        QFile f(screens);
        f.open(QIODevice::ReadOnly | QIODevice::Text);
        const QString body = QString::fromUtf8(f.readAll());
        QVERIFY(body.contains(QStringLiteral("<source>Inbox</source>")));
        QVERIFY(body.contains(QStringLiteral("Входящие")));
        QVERIFY(body.contains(QStringLiteral("<source>Settings</source>")));
        QVERIFY(body.contains(QStringLiteral("Настройки")));
        QVERIFY(body.contains(QStringLiteral("<source>Card note</source>")));
        QVERIFY(body.contains(QStringLiteral("Заметка к карточке")));
        QFile g(actions);
        g.open(QIODevice::ReadOnly | QIODevice::Text);
        const QString abody = QString::fromUtf8(g.readAll());
        QVERIFY(abody.contains(QStringLiteral("Архивировать в Gmail")));
        // %n plural must carry all three Russian forms.
        QVERIFY(body.contains(QStringLiteral("<numerusform>%n письмо</numerusform>")));
        QVERIFY(body.contains(QStringLiteral("<numerusform>%n письма</numerusform>")));
        QVERIFY(body.contains(QStringLiteral("<numerusform>%n писем</numerusform>")));
    }
    void belarusianCatalogComplete()
    {
        const QString qtDir = QStringLiteral(LB_QT_SOURCE_DIR);
        const QString screens = qtDir + QStringLiteral("/i18n/screens_be.ts");
        const QString actions = qtDir + QStringLiteral("/src/qml/actions_be.ts");
        for (const QString &path : {screens, actions}) {
            QFile f(path);
            QVERIFY2(f.open(QIODevice::ReadOnly | QIODevice::Text),
                     qPrintable(QStringLiteral("missing %1").arg(path)));
            const QString body = QString::fromUtf8(f.readAll());
            QVERIFY2(!body.contains(QStringLiteral("type=\"unfinished\"")),
                     qPrintable(QStringLiteral("%1 has unfinished entries").arg(path)));
        }
        // Spot-check Belarusian key strings (not English passthrough).
        QFile f(screens);
        f.open(QIODevice::ReadOnly | QIODevice::Text);
        const QString body = QString::fromUtf8(f.readAll());
        QVERIFY(body.contains(QStringLiteral("Назад да ўваходных")));
        QFile g(actions);
        g.open(QIODevice::ReadOnly | QIODevice::Text);
        const QString abody = QString::fromUtf8(g.readAll());
        QVERIFY(abody.contains(QStringLiteral("Архіваваць у Gmail")));
    }

    void androidUpdaterCatalogued()
    {
        const QString qtDir = QStringLiteral(LB_QT_SOURCE_DIR);
        const QStringList screens = {
            qtDir + QStringLiteral("/i18n/screens_ru.ts"),
            qtDir + QStringLiteral("/i18n/screens_be.ts"),
            qtDir + QStringLiteral("/i18n/screens_en.ts"),
        };
        for (const QString &path : screens) {
            QFile f(path);
            QVERIFY2(f.open(QIODevice::ReadOnly | QIODevice::Text),
                     qPrintable(QStringLiteral("missing %1").arg(path)));
            const QString body = QString::fromUtf8(f.readAll());
            const int ctxAt = body.indexOf(QStringLiteral("<name>AndroidUpdater</name>"));
            QVERIFY2(ctxAt >= 0,
                     qPrintable(QStringLiteral("%1 lacks AndroidUpdater context").arg(path)));
            const int ctxEnd = body.indexOf(QStringLiteral("</context>"), ctxAt);
            QVERIFY(ctxEnd > ctxAt);
            const QString ctx = body.mid(ctxAt, ctxEnd - ctxAt);
            QVERIFY2(!ctx.contains(QStringLiteral("type=\"unfinished\"")),
                     qPrintable(QStringLiteral("%1 AndroidUpdater has unfinished entries").arg(path)));
            QVERIFY2(ctx.contains(QStringLiteral("<source>update manifest SHA-256 is invalid</source>")),
                     qPrintable(QStringLiteral("%1 AndroidUpdater misses manifest strings").arg(path)));
        }
    }
};

QTEST_MAIN(I18nTest)
#include "tst_i18n.moc"
