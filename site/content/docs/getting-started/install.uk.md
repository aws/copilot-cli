---
title: Встановлення Copilot
---

# Встановлення Copilot

Ви можете встановити AWS Copilot за допомогою [Homebrew](https://brew.sh/) або завантаживши бінарні файли напряму.

## Homebrew 🍻

```sh
brew install aws/tap/copilot-cli
```

??? info "Ви використовуєте Rosetta на Mac з Apple silicon?"
    Якщо ваш Homebrew був встановлений з [Rosetta](https://developer.apple.com/documentation/apple-silicon/about-the-rosetta-translation-environment), то `brew install` встановить збірку amd64.
    Якщо це не те, що вам потрібно, будь ласка, перевстановіть Homebrew без Rosetta або скористайтеся варіантом ручної установки нижче.

## Вручну

Скопіюйте та вставте команду у ваш термінал.

=== "macOS"

    | Команда для встановлення    |
    | :---------- |
    | `curl -Lo copilot https://github.com/aws/copilot-cli/releases/latest/download/copilot-darwin && chmod +x copilot && sudo mv copilot /usr/local/bin/copilot && copilot --help` |

=== "Linux x86 (64-bit)"

    | Команда для встановлення    |
    | :---------- |
    | `curl -Lo copilot https://github.com/aws/copilot-cli/releases/latest/download/copilot-linux && chmod +x copilot && sudo mv copilot /usr/local/bin/copilot && copilot --help` |

=== "Linux (ARM)"

    | Команда для встановлення    |
    | :---------- |
    | `curl -Lo copilot https://github.com/aws/copilot-cli/releases/latest/download/copilot-linux-arm64 && chmod +x copilot && sudo mv copilot /usr/local/bin/copilot && copilot --help` |

=== "Windows"

    | Команда для встановлення    |
    | :---------- |
    | `Invoke-WebRequest -OutFile 'C:\Program Files\copilot.exe' https://github.com/aws/copilot-cli/releases/latest/download/copilot-windows.exe` |

    !!! tip "Порада"
        Будь ласка, використовуйте [Windows Terminal](https://github.com/microsoft/terminal) отримати очікуваний результат. Якщо ви зіткнулися з проблемами з дозволами, переконайтеся, що ви запускаєте термінал від імені адміністратора.

!!! info "Відомості"
    Щоб завантажити конкретну версію, замініть "latest" на конкретну версію. Наприклад, щоб завантажити v0.6.0 на macOS, введіть:
    ```
    curl -Lo copilot https://github.com/aws/copilot-cli/releases/download/v0.6.0/copilot-darwin && chmod +x copilot && sudo mv copilot /usr/local/bin/copilot && copilot --help
    ```
