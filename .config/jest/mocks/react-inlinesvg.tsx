// Due to the grafana/ui Icon component making fetch requests to
// `/public/img/icon/<icon_name>.svg` we need to mock react-inlinesvg to prevent
// the failed fetch requests from displaying errors in console.

import { Ref } from 'react';

export default function ReactInlineSVG({
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

export const cacheStore = {};