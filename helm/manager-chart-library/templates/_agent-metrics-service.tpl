{{- define "manager-chart-library.agent-metrics-service" -}}
apiVersion: v1
kind: Service
metadata:
  name: {{ .Values.agentmetricsservice.name }}
  namespace: {{ .Values.agentmetricsservice.namespace }}
spec:
  selector:
    name: {{ .Values.agentmetricsservice.selectorname }}
  ports:
    - protocol: TCP
      port: {{ .Values.agentmetricsservice.port }}
      targetPort: {{ .Values.agentmetricsservice.targetport }}
  internalTrafficPolicy: Local
{{- end -}}
