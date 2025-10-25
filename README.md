# GoVNA - Multi-Protocol NanoVNA Library for Go

GoVNA is a high-performance, thread-safe, and extensible library in Go for working with various families of NanoVNA vector network analyzers.

The project is built on a driver-based architecture (the "Bridge" design pattern), which allows for the easy addition of new device support without altering the main API.

## Features

-   **Multi-Protocol Support**: Features implemented support for both the text-based protocol of **NanoVNA V1** and the binary protocol of **NanoVNA V2/LiteVNA**.
-   **Automatic Device Detection**: The library automatically identifies the connected device type and selects the appropriate driver.
-   **Thread Safety**: Safe for use in multi-threaded applications, featuring a device pool and mutexes for synchronization.
-   **Ease of Use**: A unified API for working with any supported device.
-   **Extensibility**: New drivers for future devices can be added with ease.
-   **Performance**: Optimized to handle a large number of concurrent connections. For V2, an efficient mode is used to read all data points in a single request.
-   **Touchstone Export & VSWR**: Provides utilities to export sweeps in Touchstone format and compute VSWR values.
-   **Monitoring**: The example HTTP server exposes Prometheus metrics for scan durations.

## Supported Devices

| Device Family / Model                                    | Support Status         | Comment                                                              |
|----------------------------------------------------------|------------------------|----------------------------------------------------------------------|
| **NanoVNA V1** (H, H4, and high-quality clones)          | ✅ **Full Support**    | The text-based protocol is implemented.                              |
| **NanoVNA V2** (V2, Plus4, Plus4 Pro) / **LiteVNA** (64) | ✅ **Full Support**    | The binary protocol is implemented with a dedicated parser tailored to the device format. |
| **Clones**                                               | ✅ **Partial Support** | The device will work if its protocol is compatible with V1 or V2.    |

## Comparative Analysis

| Criterion                       | GoVNA (Go)                                                                                        | PyVNA (Python)                                                                                     | pynanovna (Python)                                                                                   |
|---------------------------------|---------------------------------------------------------------------------------------------------|----------------------------------------------------------------------------------------------------|------------------------------------------------------------------------------------------------------|
| **Driver Architecture**         | Cleanly separated: `Driver` interface, `V1Driver`, `V2Driver`, driver factory, and device pool.   | Similar to GoVNA, using ABCs and Python's dynamic typing.                                          | Less modular; drivers are not always clearly separated, often resulting in monolithic code.          |
| **Protocol Handling**           | V1 is text-based; V2 is binary with precise parsing, optimized for speed.                         | A full port of GoVNA, preserving both binary parsing and the text-based protocol.                  | Primarily uses the text-based protocol; binary parsing is partially implemented and less optimized.  |
| **Error Handling and Security** | Validates device responses and wraps driver errors; rate limiting and privilege isolation are delegated to the surrounding infrastructure. | Implements similar security measures adapted for Python.                                           | Lacks explicit protection against DoS, errors, and input validation; oriented toward local use.      |
| **Concurrency and Scalability** | Utilizes goroutines and a device pool for high scalability.                                       | Employs threads and locks, with concurrency limited by the GIL, resulting in moderate scalability. | Limited scalability; not optimized for multi-threaded operation.                                     |
| **Integration**                 | Easily integrates with cloud services, microservices, and Prometheus.                             | Integrates excellently with the Python scientific stack (NumPy, SciPy, Pandas).                    | Offers broad support for visualization and calibration but is less focused on server-side scenarios. |
| **Documentation and Support**   | Detailed, with in-code comments and examples.                                                     | Detailed, with in-code comments and examples; well-documented.                                     | Good documentation with many examples, but features a less formalized architecture.                  |

## Installation

Install the module in your project:

```bash
go get github.com/momentics/govna
```

To run the demo HTTP server that exposes Prometheus metrics:

```bash
go run ./cmd/server
```


***

GoVNA — это высокопроизводительная, потокобезопасная и расширяемая библиотека на Go для работы с различными семействами векторных анализаторов цепей NanoVNA.

Проект построен на основе драйверной архитектуры (паттерн "Мост"), что позволяет легко добавлять поддержку новых устройств, не изменяя основной API.

## Установка

Добавьте модуль в проект:

```bash
go get github.com/momentics/govna
```

Для запуска демонстрационного HTTP-сервера с метриками Prometheus выполните:

```bash
go run ./cmd/server
```

## Основные возможности

-   **Мультипротокольная поддержка**: Реализована поддержка как текстового протокола **NanoVNA V1**, так и бинарного протокола **NanoVNA V2/LiteVNA**.
-   **Автоматическое определение устройства**: Библиотека самостоятельно определяет тип подключенного устройства и выбирает нужный драйвер.
-   **Потокобезопасность**: Безопасное использование в многопоточных приложениях благодаря пулу устройств и мьютексам.
-   **Простота использования**: Единый API для работы с любым поддерживаемым устройством.
-   **Расширяемость**: Легкое добавление новых драйверов для поддержки будущих устройств.
-   **Производительность**: Оптимизировано для работы с большим количеством одновременных подключений. Для V2 используется эффективный режим чтения всех точек за один запрос.
-   **Экспорт Touchstone и VSWR**: Предоставляет утилиты для экспорта свипов в формат Touchstone и расчета коэффициента стоячей волны (VSWR).
-   **Мониторинг**: Пример HTTP-сервера публикует метрики Prometheus о длительности сканирования.
-   **Документация и тесты**: Подробные комментарии в коде и модульные тесты демонстрируют работу драйверов и калибровки.

## Поддерживаемые устройства

| Семейство/Устройство                                     | Статус поддержки           | Комментарий                                                         |
|----------------------------------------------------------|----------------------------|---------------------------------------------------------------------|
| **NanoVNA V1** (H, H4, качественные клоны)               | ✅ **Полная поддержка**    | Реализован текстовый протокол.                                      |
| **NanoVNA V2** (V2, Plus4, Plus4 Pro) / **LiteVNA** (64) | ✅ **Полная поддержка**    | Бинарный протокол реализован собственным парсером под формат устройства. |
| **Клоны**                                                | ✅ **Частичная поддержка** | Устройство будет работать, если его протокол совместим с V1 или V2. |

## Сравнительный анализ

| Аспект                              | GoVNA (Go)                                                                                        | PyVNA (Python)                                                                                      | pynanovna (Python)                                                                           |
|-------------------------------------|---------------------------------------------------------------------------------------------------|-----------------------------------------------------------------------------------------------------|----------------------------------------------------------------------------------------------|
| **Архитектура драйверов**           | Четко разделена: `Driver` interface, `V1Driver`, `V2Driver`, фабрика драйверов, пул устройств.    | Аналогично GoVNA, с использованием ABC и динамической типизации Python.                             | Меньше модульности, драйверы не всегда четко отделены, часто монолитный код.                 |
| **Обработка протоколов**            | V1 — текстовый, V2 — бинарный с точным парсингом, оптимизировано для скорости.                    | Полный порт GoVNA с сохранением бинарного парсинга и текстового протокола.                          | В основном текстовый протокол, бинарный парсинг реализован частично, с меньшей оптимизацией. |
| **Обработка ошибок и безопасность** | Проверяет ответы устройств и оборачивает ошибки драйверов; ограничения скорости и права пользователя настраиваются внешними средствами. | Аналогичные меры безопасности, адаптированные под Python.                                           | Без явной защиты от DoS, ошибок и валидации, ориентирован на локальное использование.        |
| **Обработка ошибок и безопасность** | Строгая валидация, маскировка ошибок, rate limiting, запуск от непривилегированного пользователя. | Аналогичные меры безопасности, адаптированные под Python.                                           | Без явной защиты от DoS, ошибок и валидации, ориентирован на локальное использование.        |
| **Параллелизм и масштабируемость**  | Использование горутин, пул устройств, высокая масштабируемость.                                   | Использование потоков и блокировок, ограниченный параллелизм из-за GIL, умеренная масштабируемость. | Ограниченная масштабируемость, не оптимизирован для многопоточной работы.                    |
| **Интеграция**                      | Легко интегрируется в облачные сервисы, микросервисы, Prometheus.                                 | Отлично интегрируется с научным стеком Python (NumPy, SciPy, Pandas).                               | Широкая поддержка визуализации, калибровки, но менее ориентирована на серверные сценарии.    |
| **Документация и поддержка**        | Подробная, с комментариями и примерами.                                                           | Подробная, с комментариями и примерами, хорошо документирована.                                     | Хорошая документация, много примеров, но менее формализованная архитектура.                  |
