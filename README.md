# Amazon CloudWatch data source

CloudWatch is a service that monitors applications, responds to performance changes, optimizes resource use, and provides insights into operational health. With Grafana CloudWatch plugin you can visualize logs and metrics stored in AWS CloudWatch.

This readme will guide you through setting up and authenticating your CloudWatch data source as well as querying and setting up template variables.

## Contents

- [Configure the Amazon CloudWatch data source](#configure-the-amazon-cloudwatch-data-source)
  - [Before you begin](#before-you-begin)
  - [Add the CloudWatch data source](#add-the-cloudwatch-data-source)
  - [Configure the data source in the UI](#configure-the-data-source-in-the-ui)
    - [IAM policy examples](#iam-policy-examples)
    - [Cross-account observability permissions](#cross-account-observability-permissions)
  - [Configure the data source with grafana.ini](#configure-the-data-source-with-grafanaini)
  - [Provision the data source](#provision-the-data-source)
- [Amazon CloudWatch query editor](#amazon-cloudwatch-query-editor)
  - [Choose a query editing mode](#choose-a-query-editing-mode)
  - [CloudWatch Metrics query editor components](#cloudwatch-metrics-query-editor-components)
  - [Use Builder mode](#use-builder-mode)
  - [Use Code mode](#use-code-mode)
    - [Metrics editor](#metrics-editor)
    - [Logs editor](#logs-editor)
  - [Query CloudWatch metrics](#query-cloudwatch-metrics)
    - [Metric Search queries](#metric-search-queries)
    - [Create dynamic queries with dimension wildcards](#create-dynamic-queries-with-dimension-wildcards)
    - [Use multi-value template variables](#use-multi-value-template-variables)
    - [Use metric math expressions](#use-metric-math-expressions)
    - [Period macro](#period-macro)
  - [Create queries for alerting](#create-queries-for-alerting)
    - [Deep-link Grafana panels to the CloudWatch console](#deep-link-grafana-panels-to-the-cloudwatch-console)
    - [Metric Insights queries](#metric-insights-queries)
      - [Use Metric Insights syntax](#use-metric-insights-syntax)
      - [Use Metrics Insights keywords](#use-metrics-insights-keywords)
  - [Query CloudWatch Logs](#query-cloudwatch-logs)
    - [Query Log groups with OpenSearch SQL](#query-log-groups-with-opensearch-sql)
  - [Cross-account observability](#cross-account-observability)
  - [Query caching](#query-caching)
- [CloudWatch template variables](#cloudwatch-template-variables)
  - [Use query variables](#use-query-variables)
    - [Use variables in queries](#use-variables-in-queries)
  - [Use ec2_instance_attribute](#use-ec2_instance_attribute)
    - [Filters](#filters)
    - [Select attributes](#select-attributes)


# Configure the Amazon CloudWatch data source

This document provides instructions for configuring the Amazon CloudWatch data source and explains available configuration options. For general information on adding and managing data sources, refer to [Data source management](ref:data-source-management).

## Before you begin

- You must have the `Organization administrator` role to configure the CloudWatch data source. Organization administrators can also [configure the data source via YAML](#provision-the-data-source) with the Grafana provisioning system.

- Grafana comes with a built-in CloudWatch data source plugin, so you do not need to install a plugin.

- Familiarize yourself with your CloudWatch security configuration and gather any necessary security certificates, client certificates, and client keys.

## Add the CloudWatch data source

Complete the following steps to set up a new CloudWatch data source: // todo how will the datasoruce be installed?

1. Click **Connections** in the left-side menu.
1. Click **Add new connection**
1. Type `CloudWatch` in the search bar.
1. Select the **CloudWatch data source**.
1. Click **Add new data source** in the upper right.

Grafana takes you to the **Settings** tab, where you will set up your CloudWatch configuration.

## Configure the data source in the UI

The following are configuration options for the CloudWatch data source.

| **Setting** | **Description**                                                                                                                            |
| ----------- | ------------------------------------------------------------------------------------------------------------------------------------------ |
| **Name**    | The data source name. Sets the name you use to refer to the data source in panels and queries.                                             |
| **Default** | Toggle to select as the default name in dashboard panels. When you go to a dashboard panel, this will be the default selected data source. |

Grafana plugin requests to AWS are made on behalf of an AWS Identity and Access Management (IAM) role or IAM user.
The IAM user or IAM role must have the associated policies to perform certain API actions.

For authentication options and configuration details, refer to [AWS authentication](aws-authentication/). // todo where to link here?

| Setting            | Description                                                                                                                                                                                                                  |
| ------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Authentication** | Specify which AWS credentials chain to use. A Grafana plugin's requests to AWS are made on behalf of an IAM role or IAM user. The IAM user or IAM role must have the necessary policies to perform the required API actions. |

**Access & secret key:**

You must use both an access key ID and a secret access key to authenticate.

| Setting               | Description                  |
| --------------------- | ---------------------------- |
| **Access Key ID**     | Enter your key ID.           |
| **Secret Access Key** | Enter the secret access key. |

**Assume Role**:

| Setting             | Description                                                                                                                                          |
| ------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Assume Role ARN** | _Optional._ Specify the ARN of an IAM role to assume. This ensures the selected authentication method is used to assume the role, not used directly. |
| **External ID**     | If you're assuming a role in another AWS account that requires an external ID, specify it here.                                                      |

**Additional Settings:**

| Setting                          | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
| -------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Endpoint**                     | _Optional_. Specify a custom endpoint for the AWS service.                                                                                                                                                                                                                                                                                                                                                                                                     |
| **Default Region**               | Specify the AWS region. Example: If the region is US West (Oregon), use `us-west-2`.                                                                                                                                                                                                                                                                                                                                                                           |
| **Namespaces of Custom Metrics** | Add one or more custom metric namespaces, separated by commas (for example,`Namespace1,Namespace2`). Grafana can't automatically load custom namespaces using the [CloudWatch GetMetricData API](https://docs.aws.amazon.com/AmazonCloudWatch/latest/APIReference/API_GetMetricData.html). To make custom metrics available in the query editor, manually specify the namespaces in the `Namespaces of Custom Metrics` field in the data source configuration. |

**CloudWatch Logs**:

| Setting                  | Description                                                                                                                                                                                                                                                                                                                   |
| ------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Query timeout result** | Grafana polls CloudWatch Logs every second until AWS returns a `Done` status or the timeout is reached. An error is returned if the timeout is exceeded. For alerting, the timeout defined in the Grafana config file takes precedence. Enter a valid duration string, such as `30m`, `30s` or `200ms`. The default is `30m`. |
| **Default Log Groups**   | _Optional_. Specify the default log groups for CloudWatch Logs queries.                                                                                                                                                                                                                                                       |

**X-Ray trace link:**

| Setting         | Description                                           |
| --------------- | ----------------------------------------------------- |
| **Data source** | Select the X-Ray data source from the drop-down menu. |

Grafana automatically creates a link to a trace in X-Ray data source if logs contain the `@xrayTraceId` field. To use this feature, you must already have an X-Ray data source configured. For details, see the [X-Ray data source docs](https://grafana.com/grafana/plugins/grafana-x-ray-datasource/). To view the X-Ray link, select the log row in either the Explore view or dashboard [Logs panel](ref:logs) to view the log details section.

To log the `@xrayTraceId`, refer to the [AWS X-Ray documentation](https://docs.amazonaws.cn/en_us/xray/latest/devguide/xray-services.html). To provide the field to Grafana, your log queries must also contain the `@xrayTraceId` field, for example by using the query `fields @message, @xrayTraceId`.

**Private data source connect** - _Only for Grafana Cloud users._

| Setting                         | Description                                                                                                                                                                                                                                                                                                                                                                                                                  |
| ------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Private data source connect** | Establishes a private, secured connection between a Grafana Cloud stack and data sources within a private network. Use the drop-down to locate the PDC URL. For setup instructions, refer to [Private data source connect (PDC)](https://grafana.com/docs/grafana-cloud/connect-externally-hosted/private-data-source-connect/) and [Configure PDC](https://grafana.com/docs/grafana-cloud/connect-externally-hosted/private-data-source-connect/configure-pdc/). Click **Manage private data source connect** to open your PDC connection page and view your configuration details. |

After configuring your Amazon CloudWatch data source options, click **Save & test** at the bottom to test the connection. You should see a confirmation dialog box that says:

{{< figure src="/media/docs/cloudwatch/cloudwatch-config-success-message.png" >}}


{{< admonition type="note" >}}
To troubleshoot issues while setting up the CloudWatch data source, check the `/var/log/grafana/grafana.log` file.
{{< /admonition >}}

### IAM policy examples

To read CloudWatch metrics and EC2 tags, instances, regions, and alarms, you must grant Grafana permissions via IAM.
You can attach these permissions to the IAM role or IAM user you configured in [AWS authentication](aws-authentication/). // todo

**Metrics-only permissions:**

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "AllowReadingMetricsFromCloudWatch",
      "Effect": "Allow",
      "Action": [
        "cloudwatch:DescribeAlarmsForMetric",
        "cloudwatch:DescribeAlarmHistory",
        "cloudwatch:DescribeAlarms",
        "cloudwatch:ListMetrics",
        "cloudwatch:GetMetricData",
        "cloudwatch:GetInsightRuleReport"
      ],
      "Resource": "*"
    },
    {
      "Sid": "AllowReadingTagsInstancesRegionsFromEC2",
      "Effect": "Allow",
      "Action": ["ec2:DescribeTags", "ec2:DescribeInstances", "ec2:DescribeRegions"],
      "Resource": "*"
    },
    {
      "Sid": "AllowReadingResourcesForTags",
      "Effect": "Allow",
      "Action": "tag:GetResources",
      "Resource": "*"
    },
    {
      "Sid": "AllowReadingResourceMetricsFromPerformanceInsights",
      "Effect": "Allow",
      "Action": "pi:GetResourceMetrics",
      "Resource": "*"
    }
  ]
}
```

**Logs-only permissions:**

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "AllowReadingLogsFromCloudWatch",
      "Effect": "Allow",
      "Action": [
        "logs:DescribeLogGroups",
        "logs:GetLogGroupFields",
        "logs:StartQuery",
        "logs:StopQuery",
        "logs:GetQueryResults",
        "logs:GetLogEvents"
      ],
      "Resource": "*"
    },
    {
      "Sid": "AllowReadingTagsInstancesRegionsFromEC2",
      "Effect": "Allow",
      "Action": ["ec2:DescribeTags", "ec2:DescribeInstances", "ec2:DescribeRegions"],
      "Resource": "*"
    },
    {
      "Sid": "AllowReadingResourcesForTags",
      "Effect": "Allow",
      "Action": "tag:GetResources",
      "Resource": "*"
    }
  ]
}
```

**Metrics and logs permissions:**

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "AllowReadingMetricsFromCloudWatch",
      "Effect": "Allow",
      "Action": [
        "cloudwatch:DescribeAlarmsForMetric",
        "cloudwatch:DescribeAlarmHistory",
        "cloudwatch:DescribeAlarms",
        "cloudwatch:ListMetrics",
        "cloudwatch:GetMetricData",
        "cloudwatch:GetInsightRuleReport"
      ],
      "Resource": "*"
    },
    {
      "Sid": "AllowReadingResourceMetricsFromPerformanceInsights",
      "Effect": "Allow",
      "Action": "pi:GetResourceMetrics",
      "Resource": "*"
    },
    {
      "Sid": "AllowReadingLogsFromCloudWatch",
      "Effect": "Allow",
      "Action": [
        "logs:DescribeLogGroups",
        "logs:GetLogGroupFields",
        "logs:StartQuery",
        "logs:StopQuery",
        "logs:GetQueryResults",
        "logs:GetLogEvents"
      ],
      "Resource": "*"
    },
    {
      "Sid": "AllowReadingTagsInstancesRegionsFromEC2",
      "Effect": "Allow",
      "Action": ["ec2:DescribeTags", "ec2:DescribeInstances", "ec2:DescribeRegions"],
      "Resource": "*"
    },
    {
      "Sid": "AllowReadingResourcesForTags",
      "Effect": "Allow",
      "Action": "tag:GetResources",
      "Resource": "*"
    }
  ]
}
```

#### Cross-account observability permissions

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": ["oam:ListSinks", "oam:ListAttachedLinks"],
      "Effect": "Allow",
      "Resource": "*"
    }
  ]
}
```

{{< admonition type="note" >}}
Cross-account observability lets you retrieve metrics and logs across different accounts in a single region, but you can't query EC2 Instance Attributes across accounts because those come from the EC2 API and not the CloudWatch API.
{{< /admonition >}}

For more information on configuring authentication, refer to [Configure AWS authentication](// todo link ).

### Configure the data source with grafana.ini

The Grafana [configuration file](https://grafana.com/docs/grafana/latest/setup-grafana/configure-grafana/#aws) includes an `AWS` section where you can configure data source options:

| Configuration option      | Description                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| ------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `allowed_auth_providers`  | Specifies which authentication providers are allowed for the CloudWatch data source. The following providers are enabled by default in open-source Grafana: `default` (AWS SDK default), `keys` (Access and secret key), `credentials` (Credentials file), `ec2_IAM_role` (EC2 IAM role).                                                                                                                                                       |
| `assume_role_enabled`     | Allows you to disable `assume role (ARN)` in the CloudWatch data source. The assume role (ARN) is enabled by default in open-source Grafana.                                                                                                                                                                                                                                                                                                    |
| `list_metrics_page_limit` | Sets the limit of List Metrics API pages. When a custom namespace is specified in the query editor, the [List Metrics API](https://docs.aws.amazon.com/AmazonCloudWatch/latest/APIReference/API_ListMetrics.html) populates the _Metrics_ field and _Dimension_ fields. The API is paginated and returns up to 500 results per page, and the data source also limits the number of pages to 500 by default. This setting customizes that limit. |

### Provision the data source

You can define and configure the data source in YAML files as part of the Grafana provisioning system.
For more information about provisioning and available configuration options, refer to [Provision Grafana](https://grafana.com/docs/grafana/latest/administration/provisioning/#data-sources).

**Using AWS SDK (default)**:

```yaml
apiVersion: 1
datasources:
  - name: CloudWatch
    type: cloudwatch
    jsonData:
      authType: default
      defaultRegion: eu-west-2
```

**Using credentials' profile name (non-default)**:

```yaml
apiVersion: 1

datasources:
  - name: CloudWatch
    type: cloudwatch
    jsonData:
      authType: credentials
      defaultRegion: eu-west-2
      customMetricsNamespaces: 'CWAgent,CustomNameSpace'
      profile: secondary
```

**Using `accessKey` and `secretKey`**:

```yaml
apiVersion: 1

datasources:
  - name: CloudWatch
    type: cloudwatch
    jsonData:
      authType: keys
      defaultRegion: eu-west-2
    secureJsonData:
      accessKey: '<your access key>'
      secretKey: '<your secret key>'
```

**Using AWS SDK Default and ARN of IAM Role to Assume:**

```yaml
apiVersion: 1
datasources:
  - name: CloudWatch
    type: cloudwatch
    jsonData:
      authType: default
      assumeRoleArn: arn:aws:iam::123456789012:root
      defaultRegion: eu-west-2
```

# Amazon CloudWatch query editor

Grafana provides a query editor for the CloudWatch data source, which allows you to query, visualize, and alert on logs and metrics stored in Amazon CloudWatch. It is located on the [Explore](https://grafana.com/docs/grafana/latest/explore/) page. For general documentation on querying data sources in Grafana, refer to [Query and transform data](https://grafana.com/docs/grafana/latest/panels-visualizations/query-transform-data/).

## Choose a query editing mode

The CloudWatch data source can query data from both CloudWatch metrics and CloudWatch Logs APIs, each with its own specialized query editor.

- [CloudWatch Metrics](#query-cloudwatch-metrics)
- [CloudWatch Logs](#query-cloudwatch-logs)

Select the API to query using the drop-down to the right of the **Region** setting.

## CloudWatch Metrics query editor components

The following are the components of the CloudWatch query editor.

| **Setting**     | **Description**                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
| --------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Region**      | Select an AWS region if it differs from the default.                                                                                                                                                                                                                                                                                                                                                                                                                      |
| **Namespace**   | The AWS service namespace. Examples: `AWS/EC2`, `AWS_Lambda`.                                                                                                                                                                                                                                                                                                                                                                                                             |
| **Metric name** | The name of the metric you want to visualize. Example: `CPUUtilization`.                                                                                                                                                                                                                                                                                                                                                                                                  |
| **Statistic**   | Choose how to aggregate your data. Examples: `sum`, `average`, `maximum`.                                                                                                                                                                                                                                                                                                                                                                                                 |
| **Dimensions**  | Select dimensions from the drop-down. Examples: `InstanceId`, `FunctionName`, `latency`. You can add several dimensions to your query.                                                                                                                                                                                                                                                                                                                                    |
| **Match exact** | _Optional_. When enabled, this option restricts query results to metrics that precisely match the specified dimensions and their values. All dimensions of the queried metric must be explicitly defined in the query to ensure an exact schema match. If disabled, the query will also return metrics that match the defined schema but possess additional dimensions.                                                                                                   |
| **ID**          | _Optional_. Unique identifier required by the GetMetricData API for referencing queries in math expressions. Must start with a lowercase letter and can include letters, numbers, and underscores. If not specified, Grafana generates an ID using the pattern `query[refId]` (for example, `queryA`for the first query row).                                                                                                                                             |
| **Period**      | The minimum time interval, in seconds, between data points. The default is `auto`. Valid values are 1, 5, 10, 30, or any multiple of 60. When set to auto or left blank, Grafana calculates the period using time range in seconds / 2000, then rounds up to the next value (60, 300, 900, 3600, 21600, 86400) based on [Cloudwatch's retention policy](https://aws.amazon.com/about-aws/whats-new/2016/11/cloudwatch-extends-metrics-retention-and-new-user-interface/). |
| **Label**       | _Optional_. Add a customized time series legend name. The label field overrides the default metric legend name using CloudWatch dynamic labels. Time-based dynamic labels like ${MIN_MAX_TIME_RANGE} derive legend values from the current timezone in the time range picker. For the full list of label patterns and limitations, refer to [CloudWatch dynamic labels](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/graph-dynamic-labels.html).        |

## Use Builder mode

Create a query in Builder mode:

1. Browse and select a metric namespace, metric name, filter, group, and order options using information from the [Metrics Insights keywords table](#metrics-insights-keywords).
1. For each of these keywords, choose from the list of available options.

Grafana constructs a SQL query based on your selections.

## Use Code mode

You can also write your SQL query directly in a code editor by using Code mode.

The code editor includes a built-in autocomplete feature that suggests keywords, aggregations, namespaces, metrics, labels, and label values.
Suggestions appear after typing a space, comma, or dollar (`$`) character, or by pressing <key>CTRL</key>+<key>Space</key>.

{{< admonition type="note" >}}
Template variables in the code editor can interfere with autocompletion.
{{< /admonition >}}

To run the query, click **Run query** above the code editor.

### Metrics editor

When you select `Builder` mode within the Metric search editor, a new Account field is displayed. Use the Account field to specify which of the linked accounts to target for the given query. By default, the `All` option is specified, which will target all linked accounts.

While in `Code` mode, you can specify any math expression. If the Monitoring account badge displays in the query editor header, all `SEARCH` expressions entered in this field will be cross-account by default. You can limit the search to one or a set of accounts, as documented in the [AWS documentation](http://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch-Unified-Cross-Account.html).

### Logs editor

The Log group selector specifies the target log groups for the logs query. When the Monitoring account badge appears in the query editor header, you can search and select log groups across multiple accounts. Use the `Account` field to filter log groups by account. For large numbers of log groups, use prefix search to narrow the selection.

## Query CloudWatch metrics

You can create two types of queries in the CloudWatch query editor:

- [Metric Search](#metric-search-queries), which help you retrieve and filter available metrics.
- [Metric Query](#create-a-metric-insights-queries), which use the Metrics Insights feature to fetch time series data.

The query type you use depends on how you want to interact with AWS metrics. Use the drop-down in the upper middle of the query editor to select which type you want to create.

### Metric Search queries

Metric search queries help you discover and filter available metrics. These queries use wildcards and filters to find metrics without needing to know exact metric names.

A valid metric query requires a specified namespace, metric name, and at least one statistic. Dimensions are optional, but if included, you must provide both a `key` and a `value`.

The `Match Exact` option controls how dimension filtering is applied to metric queries. When you enable `match exact`, the query returns only metrics whose dimensions precisely match the specified criteria.

This requires the following:

- All dimensions present on the target metric must be explicitly specified.
- Dimensions you don't want to filter by must use a wildcard (\*) filter.
- The metric schema must match exactly as defined in the [CloudWatch metric schema](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/search-expression-syntax.html) documentation.

When `Match Exact` is disabled, you can specify any subset of dimensions for filtering. The query returns metrics that:

- Match the specified namespace and metric name.
- Match all defined dimension filters.
- May contain additional dimensions beyond those specified.

This mode provides more flexible querying but may return metrics with unexpected additional dimensions.

The data source returns up to 100 metrics matching your filter criteria.

Enhance metric queries using template variables to create dynamic, reusable dashboards.

### Create dynamic queries with dimension wildcards

Use the asterisk (`*`) wildcard for dimension values to create dynamic queries that automatically monitor changing sets of AWS resources.

{{< figure src="/static/img/docs/cloudwatch/cloudwatch-dimension-wildcard-8.3.0.png" max-width="500px" caption="CloudWatch dimension wildcard" >}}

The query returns the average CPU utilization for all EC2 instances in the default region. With `Match Exact` disabled and `InstanceId` using a wildcard, the query retrieves metrics for any EC2 instance regardless of additional dimensions.

Auto-scaling events add new instances to the graph without manual instance ID tracking. This feature supports up to 100 metrics.

Click the [**Query inspector**](ref:query-transform-data-navigate-the-query-tab) button and select **Meta Data** to see the search expression that's automatically built to support wildcards.

To learn more about search expressions, refer to the [CloudWatch documentation](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/search-expression-syntax.html).
The search expression is defined by default in such a way that the queried metrics must match the defined dimension names exactly.
This means that in the example, the query returns only metrics with exactly one dimension containing the name `InstanceId`.

{{< figure src="/static/img/docs/cloudwatch/cloudwatch-meta-inspector-8.3.0.png" max-width="500px" class="docs-image--right" caption="CloudWatch Meta Inspector" >}}

Disabling `Match Exact` includes metrics with additional dimensions and creates a search expression even without wildcards. Grafana searches for any metric matching at least the namespace, metric name, and all defined dimensions.

### Use multi-value template variables

When defining dimension values based on multi-valued template variables, the data source uses a search expression to query for the matching metrics.
This enables the use of multiple template variables in one query, and also lets you use template variables for queries that have the `Match Exact` option disabled.

Search expressions are limited to 1,024 characters, so your query might fail if you have a long list of values.
We recommend using the asterisk (`*`) wildcard instead of the `All` option to query all metrics that have any value for a certain dimension name.

Multi-valued template variables are supported only for dimension values.
Using multi-valued template variables for `Region`, `Namespace`, or `Metric Name` is not supported.

### Use metric math expressions

Create new time series metrics using mathematical functions on CloudWatch metrics. This supports arithmetic operators, unary subtraction, and other functions. For available functions, refer to [AWS Metric Math](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/using-metric-math.html).

To apply arithmetic operations, assign a unique string ID to the raw metric, then reference this ID in the `Expression` field of the new metric.

{{< admonition type="note" >}}
If you use the expression field to reference another query, such as `queryA * 2`, you can't create an alert rule based on that query.
{{< /admonition >}}

### Period macro

If you use a CloudWatch [`SEARCH`](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/search-expression-syntax.html) expression, consider using the `$__period_auto` macro rather than specifying a period explicitly. The `$__period_auto` macro will resolve to a [CloudWatch period](https://docs.aws.amazon.com/AmazonCloudWatch/latest/APIReference/API_GetMetricData.html) that is suitable for the chosen time range.

## Create queries for alerting

Alerting requires queries that return numeric data, which CloudWatch Logs supports.
For example, you can enable alerts through the use of the `stats` command.

The following is a valid query for alerting on messages that include the text "Exception":

```
filter @message like /Exception/
    | stats count(*) as exceptionCount by bin(1h)
    | sort exceptionCount desc
```

{{< admonition type="note" >}}
If you receive an error like `input data must be a wide series but got ...` when trying to alert on a query, make sure that your query returns valid numeric data that can be output to a Time series panel.
{{< /admonition >}}

For more information on Grafana alerts, refer to [Alerting](https://grafana.com/docs/grafana/latest/alerting/).

### Deep-link Grafana panels to the CloudWatch console

{{< figure src="/static/img/docs/v65/cloudwatch-deep-linking.png" max-width="500px" class="docs-image--right" caption="CloudWatch deep linking" >}}

Left-clicking a time series in the panel displays a context menu with a link to `View in CloudWatch console`.
Clicking the link opens a new tab that takes you to the CloudWatch console and displays all metrics for that query.
If you're not logged in to the CloudWatch console, the link forwards you to the login page.
The link provided is valid for any account but displays the expected metrics only if you're logged in to the account that corresponds to the selected data source in Grafana.

This feature is not available for metrics based on [metric math expressions](#metric-math-expressions).

### Metric Insights queries

The Metrics Query option in the CloudWatch data source is referred to as **Metric Insights** in the AWS console.
It's a fast, flexible, SQL-based query engine that you can use to identify trends and patterns across millions of operational metrics in real time.

The metrics query editor's Metrics Query option has two editing modes:

- [Builder mode](#use-builder-mode), which provides a visual query-building interface.
- [Code mode](#use-code-mode), which provides a code editor for writing queries.

#### Use Metric Insights syntax

Metric Insights uses a dialect of SQL and this query syntax:

```sql
SELECT FUNCTION(MetricName)
FROM Namespace | SCHEMA(...)
[ WHERE labelKey OPERATOR labelValue [AND|...]]
[ GROUP BY labelKey [, ...]]
[ ORDER BY FUNCTION() [DESC | ASC] ]
[ LIMIT number]
```

For details about the Metrics Insights syntax, refer to the [AWS reference documentation](https://docs.aws.amazon.com/console/cloudwatch/metricsinsights-syntax).

For information about Metrics Insights limits, refer to the [AWS feature documentation](https://docs.aws.amazon.com/console/cloudwatch/metricsinsights).

You can also augment queries by using [template variables](#cloudwatch-template-variables).

#### Use Metrics Insights keywords

This table summarizes common Metrics Insights query keywords:

| Keyword      | Description                                                                                                                                                                                                  |
| ------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `FUNCTION`   | Required. Specifies the aggregate function to use, and also specifies the name of the metric to be queried. Valid values are AVG, COUNT, MAX, MIN, and SUM.                                                  |
| `MetricName` | Required. For example, `CPUUtilization`.                                                                                                                                                                     |
| `FROM`       | Required. Specifies the metric's source. You can specify either the metric namespace that contains the metric to be queried, or a SCHEMA table function. Namespace examples include `AWS/EC2`, `AWS/Lambda`. |
| `SCHEMA`     | Optional. Narrows the query results to only the metrics that are an exact match, or to metrics that do not match.                                                                                            |
| `WHERE`      | Optional. Filters the query results to only the metrics that match your specified expression. For example, `WHERE InstanceType != 'c3.4xlarge'`.                                                             |
| `GROUP BY`   | Optional. Groups the query results into multiple time series. For example, `GROUP BY ServiceName`.                                                                                                           |
| `ORDER BY`   | Optional. Specifies the order in which time series are returned. Options are `ASC`, `DESC`.                                                                                                                  |
| `LIMIT`      | Optional. Limits the number of time series returned.                                                                                                                                                         |

## Query CloudWatch Logs

The logs query editor helps you write CloudWatch Logs Query Language queries across specified regions and log groups.

It supports querying CloudWatch logs with three query language options:

- **Logs Insights QL** - The AWS native query language specifically designed for CloudWatch Logs. It uses a SQL-like syntax with commands like `fields`, `filter`, `stats`, and `sort`. It's optimized for CloudWatch's log structure and offers built-in functions for parsing timestamps, extracting fields from JSON logs, and performing aggregations.
- **OpenSearch PPL** - The OpenSearch query language is based on Elasticsearch's query DSL (Domain Specific Language). It uses a pipe-based syntax similar to Unix command-line tools or the Splunk search language, and supports complex boolean logic, range queries, wildcard matching, and full-text search capabilities.
- **OpenSearch SQL** - OpenSearch SQL is a query language that uses a SQL-like syntax for querying data in OpenSearch. It supports standard SQL queries and is designed for users familiar with SQL.

**Create a CloudWatch Logs query:**

1. Select a region.
1. Select **CloudWatch Logs** from the query type drop-down.
1. Select the query language you would like to use in the **Query Language** drop-down.
1. Click **Select log groups** and choose up to 20 log groups to query.
1. Use the main input area to write your logs query. Amazon CloudWatch only supports a subset of OpenSearch SQL and PPL commands. To find out more about the syntax supported, consult [Amazon CloudWatch Logs documentation](https://docs.aws.amazon.com/AmazonCloudWatch/latest/logs/CWL_AnalyzeLogData_Languages.html)

   {{< admonition type="note" >}}
   You must specify the region and log groups when querying with **Logs Insights QL** and **OpenSearch PPL**. **OpenSearch SQL** doesn't require log group selection. However, selecting log groups simplifies query writing by populating syntax suggestions with discovered log group fields.
   {{< /admonition >}}

Click **CloudWatch Logs Insights** to interactively view, search, and analyze your log data in the CloudWatch Logs Insights console. If you're not logged in to the CloudWatch console, the link forwards you to the login page.

### Query Log groups with OpenSearch SQL

When querying log groups with OpenSearch SQL, you **must** explicitly state the log group identifier or ARN in the `FROM` clause:

```sql
SELECT window.start, COUNT(*) AS exceptionCount
FROM `log_group`
WHERE `@message` LIKE '%Exception%'
```

or, when querying multiple log groups:

```sql
SELECT window.start, COUNT(*) AS exceptionCount
FROM `logGroups( logGroupIdentifier: ['LogGroup1', 'LogGroup2'])`
WHERE `@message` LIKE '%Exception%'
```

You can also write queries returning time series data by using the [`stats` command](https://docs.aws.amazon.com/AmazonCloudWatch/latest/logs/CWL_Insights-Visualizing-Log-Data.html).
When making `stats` queries in [Explore](https://grafana.com/docs/grafana/latest/explore), ensure you are in Metrics Explore mode.

## Cross-account observability

The CloudWatch plugin monitors and troubleshoots applications that span multiple accounts within a region. Cross-account observability enables seamless searching, visualization, and analysis of metrics and logs across account boundaries.

To enable cross-account observability, complete the following steps:

1. Go to the [Amazon CloudWatch documentation](http://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch-Unified-Cross-Account.html) and follow the instructions for enabling cross-account observability.

1. Add [two API actions](https://grafana.com/docs/grafana/latest/datasources/aws-cloudwatch/configure/#cross-account-observability-permissions) to the IAM policy attached to the role/user running the plugin.

Cross-account querying is available in the plugin through the **Logs**, **Metric search**, and **Metric Insights** modes.
After you have configured it, you'll see a **Monitoring account** badge in the query editor header.

{{< figure src="/static/img/docs/cloudwatch/cloudwatch-monitoring-badge-9.3.0.png" max-width="1200px" caption="Monitoring account badge" >}}

## Query caching

When you enable [query and resource caching](https://grafana.com/docs/grafana/latest/administration/data-source-management/#query-and-resource-caching), Grafana temporarily stores the results of data source queries and resource requests. Query caching is available in CloudWatch Metrics in Grafana Cloud and Grafana Enterprise. It is not available in CloudWatch Logs Insights due to how query results are polled from AWS.

# CloudWatch template variables

Instead of hard-coding details such as server, application, and sensor names in metric queries, you can use variables.
Grafana lists these variables in drop-down select boxes at the top of the dashboard to help you change the data displayed in your dashboard, and they are called template variables

<!-- Grafana refers to such variables as template variables. -->

For an introduction to templating and template variables, refer to [Templating](https://grafana.com/docs/grafana/latest/dashboards/variables/) and [Add and manage variables](https://grafana.com/docs/grafana/latest/dashboards/variables/add-template-variables/).

## Use query variables

You can specify these CloudWatch data source queries in the Variable edit view's **Query Type** field.
Use them to fill a variable's options list with values like `regions`, `namespaces`, `metric names`, and `dimension keys/values`.

| Name                        | List returned                                                                                                                                    |
| --------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------ |
| **Regions**                 | All AWS regions.                                                                                                                                 |
| **Namespaces**              | All namespaces CloudWatch supports.                                                                                                              |
| **Metrics**                 | Metrics in the namespace. (Specify region or use "default" for custom metrics.)                                                                  |
| **Dimension Keys**          | Dimension keys in the namespace.                                                                                                                 |
| **Dimension Values**        | Dimension values matching the specified `region`, `namespace`, `metric`, and `dimension_key`. Use dimension `filters` for more specific results. |
| **EBS Volume IDs**          | Volume ids matching the specified `region` and `instance_id`.                                                                                    |
| **EC2 Instance Attributes** | Attributes matching the specified `region`, `attribute_name`, and `filters`.                                                                     |
| **Resource ARNs**           | ARNs matching the specified `region`, `resource_type`, and `tags`.                                                                               |
| **Statistics**              | All standard statistics.                                                                                                                         |
| **LogGroups**               | All log groups matching the specified `region`.                                                                                                  |

For details on the available dimensions, refer to the [CloudWatch Metrics and Dimensions Reference](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CW_Support_For_AWS.html).

For details about the metrics CloudWatch provides, refer to the [CloudWatch documentation](https://docs.aws.amazon.com/AmazonCloudWatch/latest/DeveloperGuide/CW_Support_For_AWS.html).

### Use variables in queries

Use Grafana's variable syntax to include variables in queries. A query variable in dynamically retrieves values from your data source using a query.
For details, refer to the [variable syntax documentation](https://grafana.com/docs/grafana/latest/dashboards/variables/variable-syntax/).

## Use ec2_instance_attribute

The `ec2_instance_attribute` function in template variables allows Grafana to retrieve certain instance metadata from the EC2 metadata service, including `Instance ID` and `region`.

### Filters

The `ec2_instance_attribute` query takes `filters` as a filter name and a comma-separated list of values.

The `ec2_instance_attribute` query takes a `filters` parameter, where each key is a filter name (such as a tag or instance property), and each value is a comma-separated list of matching values.

You can specify [pre-defined filters of ec2:DescribeInstances](http://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeInstances.html).

### Select attributes

A query returns only one attribute per instance.
You can select any attribute that has a single value and isn't an object or array, also known as a `flat attribute`:

- `AmiLaunchIndex`
- `Architecture`
- `ClientToken`
- `EbsOptimized`
- `EnaSupport`
- `Hypervisor`
- `IamInstanceProfile`
- `ImageId`
- `InstanceId`
- `InstanceLifecycle`
- `InstanceType`
- `KernelId`
- `KeyName`
- `LaunchTime`
- `Platform`
- `PrivateDnsName`
- `PrivateIpAddress`
- `PublicDnsName`
- `PublicIpAddress`
- `RamdiskId`
- `RootDeviceName`
- `RootDeviceType`
- `SourceDestCheck`
- `SpotInstanceRequestId`
- `SriovNetSupport`
- `SubnetId`
- `VirtualizationType`
- `VpcId`

You can select tags by prepending the tag name with `Tags.`.
For example, select the tag `Name` by using `Tags.Name`.