To start the api you need:
 - a .env file with the structure like .env-example
 - start postgres database with docker or something else
 - create kubernetes cluster with kind (make build/kubernetes)
 - start api (make run/api)

After Creating a service you can access the deployment with port-forwarding
```
kubectl port-forward service/service-name NodePort:NormalPort
```

TODO:
 - Use structured logging slog
