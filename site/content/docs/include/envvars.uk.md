<div class="separator"></div>

<a id="variables" href="#variables" class="field">`variables`</a> <span class="type">Map</span>  
Пари ключ-значення, які представляють змінні середовища, що будуть передані вашому сервісу. Copilot стандартно включить для вас ряд змінних середовища.

<span class="parent-field">variables.</span><a id="variables-from-cfn" href="#variables-from-cfn" class="field">`from_cfn`</a> <span class="type">String</span>  
Назва [експорту стека CloudFormation](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/using-cfn-stack-exports.html).

<div class="separator"></div>

<a id="env_file" href="#env_file" class="field">`env_file`</a> <span class="type">String</span>  
Шлях до файлу з кореня вашого робочого простору, що містить змінні середовища для передачі основному контейнеру. Для отримання додаткової інформації про файл змінних середовища дивіться [Міркування щодо вказівки файлів змінних оточення.](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/taskdef-envfiles.html#taskdef-envfiles-considerations).
