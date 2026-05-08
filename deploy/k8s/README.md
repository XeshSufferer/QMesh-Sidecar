# QMesh Sidecar - Kubernetes Integration

## Обзор

QMesh Sidecar - это service mesh решение, работающее как sidecar-контейнер в Kubernetes подах. Оно обеспечивает:
- Прозрачное проксирование HTTP-трафика (iptables + TPROXY)
- Обнаружение сервисов через gossip-протокол (UDP 4221)
- Туннелирование HTTP-запросов через QUIC (TCP 4224)

## Архитектура

```
Pod:
  ├── init-container (qmesh-seed-discovery)
  │   └── Обнаруживает адреса других узлов через Kubernetes API
  │       и записывает их в shared volume
  │
  ├── init-container (qmesh-iptables)
  │   └── Настраивает iptables + TPROXY для прозрачного перехвата
  │       HTTP-трафика (порт 80/443) и маршрутизации через sidecar
  │
  ├── container (app)
  │   └── Основное приложение (трафик перехватывается прозрачно)
  │
  └── container (qmesh-sidecar)
      ├── Gossip-протокол (UDP 4221)
      ├── QUIC-туннель (TCP 4224)
      └── Transparent proxy (TCP 3128) - перехватывает и маршрутизирует трафик
```

## Быстрый старт

### 1. Сборка образов

```bash
# Сборка основного sidecar образа
make docker-build

# Сборка init-контейнера для iptables
make docker-build-init

# Или вручную:
docker build -t qmesh-sidecar:latest .
docker build -f deploy/k8s/init-container/Dockerfile.init -t qmesh-init:latest .
```

### 2. Деплой

```bash
# Создание headless service и RBAC
kubectl apply -f deploy/k8s/qmesh-headless-service.yaml
kubectl apply -f deploy/k8s/sidecar-rbac.yaml

# Деплой приложения с sidecar
kubectl apply -f deploy/k8s/example-deployment.yaml
```

### 3. Проверка

```bash
kubectl get pods -l app=app-with-qmesh
kubectl logs <pod-name> -c qmesh-sidecar
kubectl exec <pod-name> -c app -- curl -v http://google.com
```

## Использование в своем приложении

Добавьте в спецификацию Pod'а:

```yaml
spec:
  serviceAccountName: qmesh-sidecar
  
  initContainers:
    # 1. Обнаружение seed-узлов
    - name: qmesh-seed-discovery
      image: bitnami/kubectl:latest
      command:
        - /bin/bash
        - -c
        - |
          # Скрипт обнаружения узлов (см. example-deployment.yaml)
      env:
        - name: NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        - name: POD_IP
          valueFrom:
            fieldRef:
              fieldPath: status.podIP
      volumeMounts:
        - name: shared-data
          mountPath: /shared
      resources:
        requests:
          cpu: 10m
          memory: 16Mi
        limits:
          cpu: 50m
          memory: 32Mi
    
    # 2. Настройка iptables для прозрачного проксирования
    - name: qmesh-iptables
      image: qmesh-init:latest
      command:
        - /bin/bash
        - -c
        - |
          /usr/local/bin/setup-iptables.sh
          echo "iptables rules applied, keeping container running"
          sleep infinity
      securityContext:
        capabilities:
          add: ["NET_ADMIN", "NET_RAW"]
        runAsUser: 0
      env:
        - name: POD_IP
          valueFrom:
            fieldRef:
              fieldPath: status.podIP
        - name: PROXY_PORT
          value: "3128"
      volumeMounts:
        - name: shared-data
          mountPath: /shared
      resources:
        requests:
          cpu: 10m
          memory: 16Mi
        limits:
          cpu: 50m
          memory: 32Mi
  
  containers:
    - name: qmesh-sidecar
      image: qmesh-sidecar:latest
      ports:
        - name: gossip
          containerPort: 4221
          protocol: UDP
        - name: quic
          containerPort: 4224
          protocol: TCP
        - name: proxy
          containerPort: 3128
          protocol: TCP
      securityContext:
        capabilities:
          add: ["NET_ADMIN", "NET_RAW"]
      env:
        - name: GOSSIP_SEEDS
          value: ""
        - name: POD_IP
          valueFrom:
            fieldRef:
              fieldPath: status.podIP
      volumeMounts:
        - name: shared-data
          mountPath: /shared
          readOnly: true
      livenessProbe:
        tcpSocket:
          port: 4224
        initialDelaySeconds: 5
        periodSeconds: 10
      resources:
        requests:
          cpu: 50m
          memory: 64Mi
        limits:
          cpu: 200m
          memory: 128Mi
  
  volumes:
    - name: shared-data
      emptyDir: {}
```

## Как это работает

1. **Init-контейнер (seed-discovery)**: Использует Kubernetes API для поиска IP-адресов других sidecar-узлов через headless service
2. **Init-контейнер (iptables)**: Настраивает правила iptables с TPROXY для перехвата входящего HTTP-трафика (порт 80/443) и перенаправления его на локальный прокси (порт 3128)
  3. **Sidecar**: 
     - Слушает на порту 3128 для прозрачного проксирования
     - Использует TPROXY для получения оригинального адреса назначения
     - Маршрутизирует запросы через QUIC-туннель (порт 4224) к целевому узлу
     - Использует Trie-структуру для быстрого поиска маршрутов по URL path

## Переменные окружения

| Переменная | Описание | По умолчанию |
|-----------|----------|--------------|
| GOSSIP_SEEDS | Адреса seed-узлов (ip:port) | "" |
| POD_IP | IP-адрес пода | - |
| PROXY_PORT | Порт прозрачного прокси | "3128" |

Sidecar также читает seeds из файла `/shared/gossip-seeds`, если переменная окружения не задана.

## Порты

| Порт | Протокол | Описание |
|------|----------|----------|
| 4221 | UDP | Gossip-протокол для обнаружения узлов |
| 4224 | TCP/QUIC | Туннелирование HTTP-запросов между узлами |
| 3128 | TCP | Transparent proxy (TPROXY) для перехвата трафика |

## Принцип прозрачного проксирования

iptables правила (в init-контейнере):
```bash
# Создаем цепочку QMESH_PROXY
iptables -t mangle -N QMESH_PROXY

# Исключаем localhost и собственные адреса
iptables -t mangle -A QMESH_PROXY -d 127.0.0.1/8 -j RETURN
iptables -t mangle -A QMESH_PROXY -d ${POD_IP} -j RETURN

# Перехватываем HTTP (80) и HTTPS (443) трафик
iptables -t mangle -A QMESH_PROXY -p tcp --dport 80 -j TPROXY --on-port 3128 --tproxy-mark 0x1/0x1
iptables -t mangle -A QMESH_PROXY -p tcp --dport 443 -j TPROXY --on-port 3128 --tproxy-mark 0x1/0x1

# Применяем к OUTPUT цепочке
iptables -t mangle -A OUTPUT -j QMESH_PROXY

# Маршрутизация для TPROXY
ip rule add fwmark 0x1 lookup 100
ip route add local 0.0.0.0/0 dev lo table 100
```

## Очистка

```bash
kubectl delete -f deploy/k8s/
```

## Тестирование

```bash
# Проверка логов sidecar
kubectl logs <pod-name> -c qmesh-sidecar

# Проверка iptables правил в init-контейнере
kubectl exec <pod-name> -c qmesh-iptables -- iptables -t mangle -L -n -v

# Тест прозрачного проксирования
kubectl exec <pod-name> -c app -- curl -v http://example.com
```

## Требования

- Kubernetes 1.20+
- Доступ к Kubernetes API (для seed discovery)
- Возможность использовать NET_ADMIN и NET_RAW capabilities
- Поддержка TPROXY в ядре узла (обычно включено по умолчанию)
