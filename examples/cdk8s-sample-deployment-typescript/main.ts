import { Construct } from 'constructs';
import { ApiObject, App, Chart, ChartProps } from 'cdk8s';
import { KubeDeployment } from './imports/k8s';

export class NginxChart extends Chart {
  public static readonly CHART_NAME = 'nginx-deployment';

  constructor(scope: Construct, id: string, props: ChartProps = {}) {
    super(scope, id, props);

    const nginxLabels = { app: 'nginx' };
    new KubeDeployment(this, NginxChart.CHART_NAME, {
      metadata: {
        name: NginxChart.CHART_NAME,
        labels: nginxLabels,
        namespace: 'default',
      },
      spec: {
        replicas: 3,
        selector: {
          matchLabels: nginxLabels,
        },
        template: {
          metadata: {
            labels: nginxLabels,
          },
          spec: {
            containers: [
              {
                name: 'nginx',
                image: 'nginx:latest',
                ports: [{ containerPort: 80 }],
              },
            ],
          },
        },
      },
    });
  }
}

export class HeadlampChart extends Chart {
  public static readonly CHART_NAME = 'headlamp-deployment';
  
  constructor(scope: Construct, id: string, props: ChartProps = { }) {
    super(scope, id, props);
    const headlampLabels = { app: 'headlamp' };
    const headlampDeploymentName = HeadlampChart.CHART_NAME;
    new KubeDeployment(this, headlampDeploymentName, {
      metadata: {
        name: headlampDeploymentName,
        labels: headlampLabels,
        namespace: 'default',
      },
      spec: {
        replicas: 3,
        selector: {
          matchLabels: headlampLabels,
        },
        template: {
          metadata: {
            labels: headlampLabels,
          },
          spec: {
            containers: [
              {
                name: 'headlamp',
                image: 'ghcr.io/headlamp-k8s/headlamp:latest',
                args: ['-in-cluster', '-plugins-dir=/headlamp/plugins'],
                env: [
                  { name: 'HEADLAMP_CONFIG_TRACING_ENABLED', value: 'true' },
                  { name: 'HEADLAMP_CONFIG_METRICS_ENABLED', value: 'true' },
                  { name: 'HEADLAMP_CONFIG_OTLP_ENDPOINT', value: 'otel-collector:4317' },
                  { name: 'HEADLAMP_CONFIG_SERVICE_NAME', value: 'headlamp' },
                  { name: 'HEADLAMP_CONFIG_SERVICE_VERSION', value: 'latest' },
                ],
                ports: [
                  { containerPort: 4466, name: 'http' },
                  { containerPort: 9090, name: 'metrics' },
                ],
              },
            ],
          },
        },
      },
    });

    new ApiObject(this, 'headlamp-service', {
      apiVersion: 'v1',
      kind: 'Service',
      metadata: {
        name: 'headlamp-service',
        namespace: 'default',
      },
      spec: {
        selector: headlampLabels,
        ports: [{
          port: 80,
          targetPort: 4466,
        }],
      },
    });

    new ApiObject(this, 'headlamp-secret', {
      apiVersion: 'v1',
      kind: 'Secret',
      metadata: {
        name: 'headlamp-admin',
        namespace: 'default',
        annotations: {
          'kubernetes.io/service-account.name': 'headlamp-admin'
        },
      },
      type: 'kubernetes.io/service-account-token',
  });
}
}

export class KustomizeResources extends Chart {
  constructor(scope: Construct, id: string, props: ChartProps = {}) {
    super(scope, id, props);

    new ApiObject(this, 'kustomization', {
      apiVersion: 'kustomize.config.k8s.io/v1beta1',
      kind: 'Kustomization',
      metadata: {
        name: 'kustomization',
        namespace: 'default',
      },
        resources: [
          HeadlampChart.CHART_NAME + '.k8s.yaml',
          NginxChart.CHART_NAME + '.k8s.yaml',
        ],
    });
  }
}

const app = new App();
new NginxChart(app, 'nginx-deployment');
new HeadlampChart(app, 'headlamp-deployment');
new KustomizeResources(app, 'kustomization');
app.synth();
