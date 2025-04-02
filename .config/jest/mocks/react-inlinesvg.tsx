// Due to the grafana/ui Icon component making fetch requests to
// `/public/img/icon/<icon_name>.svg` we need to mock react-inlinesvg to prevent
// the failed fetch requests from displaying errors in console.

import React, { Ref } from 'react';

type Callback = (...args: any[]) => void;

export interface StorageItem {
  content: string;
  queue: Callback[];
  status: string;
}

export const cacheStore: { [key: string]: StorageItem } = Object.create(null);

export default function InlineSVG ({
  innerRef,
  cacheRequests,
  preProcessor,
  ...rest
}: {
  innerRef: Ref<SVGSVGElement>;
  cacheRequests: boolean;
  preProcessor: () => string;
}) {
  return <svg ref={innerRef} {...rest} />;
}
