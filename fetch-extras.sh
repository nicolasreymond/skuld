#!/bin/sh
# Récupère absences (+ bulletin) via gaps-cli, 1×/jour (discret, éviter un ban GAPS).
# L'écriture se fait DANS le conteneur (qui possède /history en écriture).
docker exec skuld-scraper sh -c '/gaps-cli absences --format json > /history/absences.json.tmp && mv /history/absences.json.tmp /history/absences.json'
docker exec skuld-scraper sh -c '/gaps-cli report-card --format json > /history/reportcard.json.tmp && mv /history/reportcard.json.tmp /history/reportcard.json'
