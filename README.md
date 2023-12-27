# energomer125 reader
В папке **info** содержится информация о протоколе, который используется для получения данных с прибора Энергомер 125.
В настоящее время используется версия 1 этого протокола, но также в папке находится описание версии 2 на случай внесения изменений.

# сетевые настройки
server read data **10.21.15.177** port **52321**, **52325**
Доступ предоставлен на сервер **krr-app-pacnt08** (10.21.2.139)
где настроена переадресация портов для сервиса находящегося на **krr-app-pasvc02**

### просмотр всех правил
    netsh interface portproxy show all
### проброс порта 10.21.15.177:52321 -> 10.21.2.139 :52321

    netsh interface portproxy add v4tov4 listenport=52321 listenaddress=10.21.2.139 connectport=52321 connectaddress=10.21.15.177

### удаление порта 10.21.2.139 :52321

    netsh interface portproxy delete v4tov4 listenport=52321 listenaddress=10.21.2.139

### проброс порта 10.21.15.177:52325 -> 10.21.2.139 :52325

    netsh interface portproxy add v4tov4 listenport=52325 listenaddress=10.21.2.139 connectport=52325 connectaddress=10.21.15.177

### удаление порта 10.21.2.139 :52325

    netsh interface portproxy delete v4tov4 listenport=52325 listenaddress=10.21.2.139

# также необходимо открыть в фаерволе порт 52321, 52325 на 10.21.2.139
