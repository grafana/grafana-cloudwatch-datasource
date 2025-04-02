// Jest setup provided by Grafana scaffolding
import './.config/jest-setup';
import React from 'react';
import { matchers } from './src/test/matchers';

global.React = React;

// mock the intersection observer and just say everything is in view
const mockIntersectionObserver = jest.fn().mockImplementation((callback) => ({
    observe: jest.fn().mockImplementation((elem) => {
      callback([{ target: elem, isIntersecting: true }]);
    }),
    unobserve: jest.fn(),
    disconnect: jest.fn(),
  }));
  global.IntersectionObserver = mockIntersectionObserver;

expect.extend(matchers);
