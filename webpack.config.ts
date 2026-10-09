import CopyWebpackPlugin from 'copy-webpack-plugin';
import webpack, { type Configuration } from 'webpack';
import { merge } from 'webpack-merge';
import grafanaConfig from './.config/webpack/webpack.config';
import path from 'path';
import { SOURCE_DIR } from './.config/bundler/constants';

const config = async (env): Promise<Configuration> => {
  const baseConfig = await grafanaConfig(env);

  return merge(baseConfig, {
    module: {
      ...baseConfig.module,
      rules: [
        ...(baseConfig.module?.rules || []),
        {
          exclude: /(node_modules)/,
          test: /\.[tj]sx?$/,
          use: {
            loader: 'swc-loader',
            options: {
              jsc: {
                baseUrl: path.resolve(process.cwd(), SOURCE_DIR),
                target: 'es2015',
                loose: false,
                parser: {
                  syntax: 'typescript',
                  tsx: true,
                  decorators: false,
                  dynamicImport: true,
                },
                transform: {
                  react: {
                    runtime: 'automatic',
                  },
                },
              },
            },
          },
        },
      ],
    },
    output: {
      asyncChunks: true,
    },
    externals: ['react/jsx-runtime', 'react/jsx-dev-runtime'],
    plugins: [
      // @grafana/prometheus bundles monaco-promql which imports monaco-editor CSS files
      // directly. Grafana already ships Monaco with its own CSS, so we drop these CSS
      // side-effect imports to avoid the double css-loader processing that breaks webpack.
      new webpack.IgnorePlugin({
        resourceRegExp: /\.css$/,
        contextRegExp: /monaco-editor/,
      }),
      new CopyWebpackPlugin({
        patterns: [
          { from: '../pkg/schema/dsconfig.json', to: './schema/dsconfig.json' },
          { from: '../pkg/schema/schema.gen.json', to: './schema/v0alpha1.json' },
          { from: '../pkg/schema/settings.gen.json', to: './schema/v0alpha1/settings.json' },
          { from: '../pkg/schema/settings.examples.gen.json', to: './schema/v0alpha1/settings.examples.json' },
        ],
      }),
    ],
  });
};

export default config;
