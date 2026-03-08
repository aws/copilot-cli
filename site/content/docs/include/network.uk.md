<div class="separator"></div>

<a id="network" href="#network" class="field">`network`</a> <span class="type">Map</span>  
Розділ `network` містить параметри для підключення до ресурсів AWS у VPC.

<span class="parent-field">network.</span><a id="network-connect" href="#network-connect" class="field">`connect`</a> <span class="type">Bool або Map</span>  
Увімкніть [Service Connect](../../developing/svc-to-svc-communication/#service-connect) для вашого сервісу, що робить трафік між сервісами збалансованим і більш стійким. Стандартно `false`.

При використанні map, ви можете вказати, який псевдонім використовувати для цього сервісу. Зверніть увагу, що псевдонім має бути унікальним у межах середовища.

<span class="parent-field">network.connect.</span><a id="network-connect-alias" href="#network-connect-alias" class="field">`alias`</a> <span class="type">String</span>  
Користувацьке DNS-імʼя для цього сервісу, що експонується через Service Connect. Стандартно використовується імʼя сервісу.

{% include 'network-vpc.uk.md' %}
