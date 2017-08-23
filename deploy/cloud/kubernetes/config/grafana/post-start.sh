#!/usr/bin/env sh

DIR="$( cd "$( dirname "${0}" )" && pwd )"
GRAFANA_URL=${1}
DATASOURCES_URL="${GRAFANA_URL}/api/datasources"
DASHBOARDS_URL="${GRAFANA_URL}/api/dashboards/import"

echo -n "waiting for grafana to start ... "
until $(curl --silent --fail --show-error --output /dev/null ${DATASOURCES_URL}); do
    sleep 1
done
echo "done"

for file in ${DIR}/datasource.*.json ; do
    echo "importing datasource from ${file}"
    curl --silent --fail --show-error \
        --request POST "${DATASOURCES_URL}" \
        --header "Content-Type: application/json" \
        --data-binary "@${file}"
    echo
done

for file in ${DIR}/dashboard.*.json ; do
    echo "importing dashboard from ${file}"
    (echo '{"dashboard":';cat "${file}";echo ',"inputs":[{"name":"DS_PROMETHEUS","pluginId":"prometheus","type":"datasource","value":"prometheus"}]}') | \
    curl --silent --fail --show-error \
        --request POST ${DASHBOARDS_URL} \
        --header "Content-Type: application/json" \
        --data-binary @-;
    echo
done
