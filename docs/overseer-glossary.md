# Overseer glossary

This is the one glossary for Overseer copy. Every string in the facility workspace, and in the
dialogs it opens, names a facility entity with a term from the table below. The vocabulary check in
`tests/browser/facility-vocabulary.spec.mjs` reads the patterns at the end of this file and fails
when a banned wording appears in the Overseer sources.

The glossary governs copy only. Code names (`condition`, `binding`, `DiagnosticCondition`) and
session JSON field names stay as they are.

## Terms

The «Термин» column gives each term and, in brackets, the form controls and titles render (see
[Case convention](#case-convention)). «Не называть» lists the wordings the vocabulary check rejects
for that term. A dash means the check rejects no synonym for it.

| Термин | Что это | Не называть |
|---|---|---|
| Локация («ЛОКАЦИЯ»; в подписях «…ЛОКАЦИИ», в предложениях склоняется: «в локации») | Вся сцена: устройства, неисправности и программы восстановления (`domain.Facility`, поле `facility`) вместе с терминалами, которыми они управляют (`terminals[]`). | «объект» в любой форме, кроме [разрешённых строк](#allowed-strings) |
| Устройство («УСТРОЙСТВО») | Одна часть локации со своими состояниями и переходами, например «Дверь в убежище» (`FacilityDevice`, `facility.devices[]`). | «объект» в любой форме, кроме [разрешённых строк](#allowed-strings) |
| Состояние («СОСТОЯНИЕ», «СОСТОЯНИЯ») | Одно из положений устройства; начальное состояние отмечено звездой на своём чипе (`FacilityDeviceState`, `facility.devices[].states[]`). | — |
| Переход («ПЕРЕХОД», «ПЕРЕХОДЫ», «ПЕРЕХОД: <из> → <в>») | Именованная смена одного состояния устройства на другое, с предусловиями и влиянием на неисправности (`FacilityDeviceTransition`, `facility.devices[].transitions[]`). | — |
| Правило отображения («ПРАВИЛО ОТОБРАЖЕНИЯ») | Правило терминала, которое по состоянию устройства меняет текст записи или название в меню, показывает элемент или разрешает команду (один `FacilityTextVariant` в `facilityNameVariants[]` или `blocks[].facilityTextVariants[]`, либо одно равенство `visibleWhen` или `availableWhen` в узле `terminals[].root`). | «привязка объекта» |
| Действие команды («ДЕЙСТВИЕ КОМАНДЫ») | Переходы или программа восстановления, которые запускает команда терминала; настраивается только в диалоге «ДЕЙСТВИЕ КОМАНДЫ» (`StateChangeConfig.FacilityAction`, `stateChange.facilityAction` команды). | — |
| Неисправность («НЕИСПРАВНОСТЬ», «НЕИСПРАВНОСТИ») | Сбой, который включают и снимают переходы, программы восстановления или Смотритель; относится к одному устройству или к одному терминалу (`DiagnosticCondition`, `facility.conditions[]`). Неисправности терминалов собраны в группе «НЕИСПРАВНОСТИ ТЕРМИНАЛОВ». | «диагностическое условие»; «условие» в любой форме, кроме слов, где перед ним стоит буква, например «предусловие» |
| Программа восстановления («ПРОГРАММА ВОССТАНОВЛЕНИЯ») | Именованный список переходов, который команда терминала запускает одним действием, чтобы снять неисправность (`RecoveryProgram`, `facility.recoveryPrograms[]`). | — |
| Смотритель вручную («Смотритель вручную») | Способ снять любую неисправность без перехода и программы: Смотритель выключает её переключателем. Всегда указан в «КАК УСТРАНЯЕТСЯ». Сохранённое значение `privateOverseerAction` остаётся в файле без изменений и больше ни на что не влияет. | «Восстановление оператором»; «Восстановить условие» |
| Готово / Отмена («ГОТОВО» / «ОТМЕНА») | Пара кнопок каждого диалога правки локации. «ГОТОВО» сохраняет черновик одной правкой с одной проверкой ревизии и одной записью «ОТМЕНИТЬ»; «ОТМЕНА» закрывает диалог и ничего не сохраняет. | — |

Other Overseer dialogs keep their own buttons («СОХРАНИТЬ…», «ЗАКРЫТЬ»), and confirmation dialogs
keep naming their action («СБРОСИТЬ», «ПЕРЕВЕСТИ»).

## Allowed strings

Two strings that contain «объект» stay, because they name the workspace itself, not a device or the
location:

- «ОБЪЕКТЫ», the workspace tab.
- «ОБЪЕКТЫ И СОСТОЯНИЯ», the workspace heading.

## Case convention

The Overseer has no CSS `text-transform`, so every string is literal in the source:

- Controls, labels, legends, dialog titles, tab names, and statuses render in upper case.
- Hints, warnings, sentences, and error messages stay in sentence case.
- Authored names (devices, states, transitions, faults, programs, terminals, elements) keep their
  case.
- «Смотритель вручную» keeps sentence case, because it is a list entry next to authored names.
- «подробности для разработчика» keeps sentence case, because it belongs to the message it follows.

`specs/041-facility-editors-language/contracts/overseer-ui.md` maps every spec string to its
rendered form.

## Vocabulary check

The check fetches the served `index.html`, `overseer.js`, and `desktop-api.js` through the fixture
server and scans their full text, comments included. Authored session content, such as the demo
folder «СИСТЕМЫ ОБЪЕКТА», is never scanned. The 039 `REMOVED_COPY` sweep stays as it is.

The check reads the two fenced blocks below:

- Each `vocabulary-banned` line is a JavaScript regular expression source, compiled with the flags
  `iu`. The patterns never use `\b`, because it does not work on Cyrillic. They use the boundary
  `(?<!\p{L})` instead, so «предусловие» passes.
- Each `vocabulary-allowed` line is a case-sensitive literal phrase. Before matching, the check
  removes each occurrence of a phrase that stands alone, longest phrase first. An occurrence stands
  alone when neither neighbouring character is a letter, a digit, or whitespace: the escaped phrase
  is wrapped in `(?<![\p{L}\p{N}\s])` and `(?![\p{L}\p{N}\s])` and compiled with the flags `gu`. So
  the tab's `>ОБЪЕКТЫ<` is removed, and «ОБЪЕКТЫ УБЕЖИЩА» is still a hit.

The check reports each hit as `<file>:<line>: <match>` and passes only with zero hits.

```vocabulary-banned
(?<!\p{L})объект\p{L}*
диагностическ\p{L}*\s+услови\p{L}*
(?<!\p{L})услови\p{L}*
восстановить\s+услови\p{L}*
восстановлени\p{L}*\s+оператором
```

```vocabulary-allowed
ОБЪЕКТЫ И СОСТОЯНИЯ
ОБЪЕКТЫ
```
