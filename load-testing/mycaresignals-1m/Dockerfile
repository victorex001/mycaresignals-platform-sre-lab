FROM grafana/k6:0.54.0

COPY k6/scenario.js /scripts/scenario.js

ENTRYPOINT ["k6"]
CMD ["run", "/scripts/scenario.js"]
