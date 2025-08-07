# Changelog

## 12.1.1

- Chore: bump jest-environment-jsdom from 30.0.4 to 30.0.5 by @dependabot[bot] in https://github.com/grafana/grafana-cloudwatch-datasource/pull/89
- Chore: bump @swc/core from 1.13.1 to 1.13.2 by @dependabot[bot] in https://github.com/grafana/grafana-cloudwatch-datasource/pull/90
- Chore: bump jest from 30.0.4 to 30.0.5 by @dependabot[bot] in https://github.com/grafana/grafana-cloudwatch-datasource/pull/88
- Chore: bump @testing-library/jest-dom from 6.6.3 to 6.6.4 by @dependabot[bot] in https://github.com/grafana/grafana-cloudwatch-datasource/pull/97
- Fix dependabot ignore location for npm dependencies by @kevinwcyu in https://github.com/grafana/grafana-cloudwatch-datasource/pull/94
- Chore: bump @swc/core from 1.13.2 to 1.13.3 by @dependabot[bot] in https://github.com/grafana/grafana-cloudwatch-datasource/pull/99
- Chore: bump @grafana/plugin-ui from 0.10.7 to 0.10.8 by @dependabot[bot] in https://github.com/grafana/grafana-cloudwatch-datasource/pull/100
- Chore: bump github.com/aws/aws-sdk-go-v2/service/oam from 1.18.3 to 1.18.4 by @dependabot[bot] in https://github.com/grafana/grafana-cloudwatch-datasource/pull/83
- Chore: bump github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi from 1.26.6 to 1.26.7 by @dependabot[bot] in https://github.com/grafana/grafana-cloudwatch-datasource/pull/84
- Chore: bump github.com/aws/aws-sdk-go-v2/service/cloudwatch from 1.45.3 to 1.45.4 by @dependabot[bot] in https://github.com/grafana/grafana-cloudwatch-datasource/pull/87
- Chore: bump github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs from 1.51.0 to 1.53.1 by @dependabot[bot] in https://github.com/grafana/grafana-cloudwatch-datasource/pull/103
- Chore: bump github.com/grafana/grafana-aws-sdk from 1.0.4 to 1.1.0 by @dependabot[bot] in https://github.com/grafana/grafana-cloudwatch-datasource/pull/102
- Chore: bump github.com/aws/smithy-go from 1.22.4 to 1.22.5 by @dependabot[bot] in https://github.com/grafana/grafana-cloudwatch-datasource/pull/101
- Chore: bump github.com/aws/aws-sdk-go-v2/service/ec2 from 1.231.0 to 1.235.0 by @dependabot[bot] in https://github.com/grafana/grafana-cloudwatch-datasource/pull/105
- Fix handling region for legacy alerts by @iwysiu in https://github.com/grafana/grafana-cloudwatch-datasource/pull/104

## 12.1.0

- Fix: use configured default region if empty region requested by @njvrzm in [#92](https://github.com/grafana/grafana-cloudwatch-datasource/pull/92)
- CloudWatch: Clear log groups when region is changed by @iwysiu [#91](https://github.com/grafana/grafana-cloudwatch-datasource/pull/91)
- Updates from core plugin by @njvrzm in [#81](https://github.com/grafana/grafana-cloudwatch-datasource/pull/81)
- Tweak dependabot schedule by @kevinwcyu in [#93](https://github.com/grafana/grafana-cloudwatch-datasource/pull/93)
- Fix: Handle log alerts queries where the time field is not the first field by @kevinwcyu in [#79](https://github.com/grafana/grafana-cloudwatch-datasource/pull/79)
- Set accountId to undefined if not monitoring account by @idastambuk in [#58](https://github.com/grafana/grafana-cloudwatch-datasource/pull/58)
- Dependabot updates:
- Chore: bump github.com/aws/aws-sdk-go-v2/service/ec2 from 1.230.0 to 1.231.0 (#80)
- Chore: bump eslint-config-prettier from 10.1.5 to 10.1.8 (#77)
- Chore: bump @types/node from 22.16.4 to 22.16.5 (#76)
- Chore: bump @grafana/plugin-e2e from 2.1.6 to 2.1.7 (#75)
- Chore: bump @swc/core from 1.13.0 to 1.13.1 (#74)
- Chore: bump @playwright/test from 1.53.2 to 1.54.1 (#72)
- Chore: bump @typescript-eslint/eslint-plugin from 8.35.0 to 8.37.0 (#73)
- Chore: bump @swc/core from 1.12.14 to 1.13.0 (#71)
- Chore: bump webpack from 5.99.9 to 5.100.2 (#70)
- Chore: bump eslint from 9.29.0 to 9.31.0 (#69)
- Chore: bump @swc/jest from 0.2.38 to 0.2.39 (#67)
- Remove @types/testing-library\_\_jest-dom (#65)
- Chore: bump eslint-plugin-jsdoc from 50.6.17 to 51.4.1 (#66)
- Chore: bump @typescript-eslint/parser from 8.35.0 to 8.37.0 (#64)
- Chore: bump @grafana/eslint-config from 8.0.0 to 8.1.0 (#68)
- Chore: bump @eslint/js from 9.29.0 to 9.31.0 (#61)
- Chore: bump @types/node from 22.15.35 to 22.16.4 (#63) Chore: bump @grafana/plugin-e2e from 2.1.3 to 2.1.6 (#62)
- Chore: bump @types/jest from 29.5.14 to 30.0.0 (#60)
- Chore: bump @swc/core from 1.12.7 to 1.12.14 (#59)
- Chore: bump jest-environment-jsdom from 29.7.0 to 30.0.0 (#50)
- Chore: bump jest from 30.0.0 to 30.0.4 (#56)
- Chore: bump glob from 11.0.2 to 11.0.3 (#52)
- Chore: bump @stylistic/eslint-plugin-ts from 4.4.0 to 4.4.1 (#57)
- Chore: bump @playwright/test from 1.53.1 to 1.53.2 (#54)
- Chore: bump jest from 29.7.0 to 30.0.0 (#55)
- Chore: bump @typescript-eslint/eslint-plugin from 8.32.1 to 8.35.0 (#53)

## 12.0.5

- Fix PDC transport issue & bump grafana-aws-sdk in [#37](https://github.com/grafana/grafana-cloudwatch-datasource/pull/34)

## 12.0.4

- Fix externalID handling by @njvrzm in [#34](https://github.com/grafana/grafana-cloudwatch-datasource/pull/34)
- Add missing regions to constants by @idastambuk in [#31](https://github.com/grafana/grafana-cloudwatch-datasource/pull/31)
- Improve instance attribute variable query editor by @idastambuk in [#32]https://github.com/grafana/grafana-cloudwatch-datasource/pull/32)
- Chore: add dependabot by @idastambuk in [#22](https://github.com/grafana/grafana-cloudwatch-datasource/pull/22)
- Dependency updates:
  - Chore: Migrate to eslintv9 and bump the node-dependency group with 41 updates by @dependabot in [#24](https://github.com/grafana/grafana-cloudwatch-datasource/pull/24)
  - Bump the all-go-dependencies group with 7 updates by @dependabot in [#23](https://github.com/grafana/grafana-cloudwatch-datasource/pull/23)

## 12.0.3

- Externalization: bump grafana-aws-sdk for assume role fix, bump plugin-sdk-go for good measure [#21](https://github.com/grafana/grafana-cloudwatch-datasource/pull/21)
-

## 12.0.2

- Externalization: Fix runtime jsx configuration in [#18](https://github.com/grafana/grafana-cloudwatch-datasource/pull/18)

## 12.0.1

- Fix: Bump grafana-aws-sdk for v2 auth fix in [#14](https://github.com/grafana/grafana-cloudwatch-datasource/pull/14)

## 12.0.0

- Initial release

## 1.0.0 (Unreleased)
