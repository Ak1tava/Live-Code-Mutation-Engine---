# Инструменты презентации

Выбор выполнен вместе с субагентом GPT-6 Astra. Canva 17.0.1 установлен через `codex plugin add canva@openai-curated-remote --json`; повторная проверка каталога подтвердила `installed: true`. Presentations уже был доступен.

- [Canva в каталоге OpenAI](https://openai.com/business/plugins/canva/): визуальные макеты и редактируемые презентации.
- [Presentations в каталоге OpenAI](https://openai.com/business/plugins/presentations/): создание, проверка и экспорт презентаций.
- [Установка плагинов OpenAI](https://help.openai.com/en/articles/20001256-plugins-in-chatgpt): установка не заменяет отдельный вход в сервис, если он потребуется.

Обсуждения пользователей сравнивали скорость черновика и качество экспорта: [Gamma](https://www.reddit.com/r/GammaApp/comments/1wjhp5c/is_gamma_better_than_powerpoint_or_just_faster_at/), [PowerPoint в Codex](https://www.reddit.com/r/codex/comments/1wu03c6/how_to_create_better_powerpoint_presentations/), [экспорт Canva](https://www.reddit.com/r/canva/comments/1udw33g/is_there_a_way_to_export_a_presentation_as_a_pptx/). Это отдельные отзывы, не репрезентативный рейтинг. Оснований устанавливать ещё несколько перекрывающих инструментов не найдено.

В этой сессии вызов Canva `create-design` вернул `Unknown tool`, несмотря на установленный плагин. Поэтому готовые PDF/PPTX созданы встроенным Presentations; установка Canva не выдаётся за успешную генерацию в Canva. Проверены структура PowerPoint, шрифты, размер слайдов, повторный импорт и визуально все шесть слайдов; PDF проверен после рендеринга.
