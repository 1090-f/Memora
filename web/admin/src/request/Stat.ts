/* eslint-disable */
/* tslint:disable */
// @ts-nocheck
/*
 * ---------------------------------------------------------------
 * ## THIS FILE WAS GENERATED VIA SWAGGER-TYPESCRIPT-API        ##
 * ##                                                           ##
 * ## AUTHOR: acacode                                           ##
 * ## SOURCE: https://github.com/acacode/swagger-typescript-api ##
 * ---------------------------------------------------------------
 */

import httpRequest, { ContentType, RequestParams } from "./httpClient";
import {
  DomainHotBrowser,
  DomainHotPage,
  DomainHotRefererHost,
  DomainInstantCountResp,
  DomainInstantPageResp,
  DomainPWResponse,
  DomainResponse,
  GetApiV1StatBrowsersParams,
  GetApiV1StatConversationDistributionParams,
  GetApiV1StatCountParams,
  GetApiV1StatGeoCountParams,
  GetApiV1StatHotPagesParams,
  GetApiV1StatInstantCountParams,
  GetApiV1StatInstantPagesParams,
  GetApiV1StatRefererHostsParams,
  V1StatConversationDistributionResp,
  V1StatCountResp,
} from "./types";

/**
 * @description 客户端统计
 *
 * @tags stat
 * @name GetApiV1StatBrowsers
 * @summary 客户端统计
 * @request GET:/api/v1/stat/browsers
 * @secure
 * @response `200` `(DomainResponse & {
    data?: DomainHotBrowser,

})` OK
 */

export const getApiV1StatBrowsers = (
  query: GetApiV1StatBrowsersParams,
  params: RequestParams = {},
) =>
  httpRequest<
    DomainResponse & {
      data?: DomainHotBrowser;
    }
  >({
    path: `/api/v1/stat/browsers`,
    method: "GET",
    query: query,
    secure: true,
    type: ContentType.Json,
    format: "json",
    ...params,
  });

/**
 * @description 问答来源
 *
 * @tags stat
 * @name GetApiV1StatConversationDistribution
 * @summary 问答来源
 * @request GET:/api/v1/stat/conversation_distribution
 * @secure
 * @response `200` `(DomainResponse & {
    data?: (V1StatConversationDistributionResp)[],

})` OK
 */

export const getApiV1StatConversationDistribution = (
  query: GetApiV1StatConversationDistributionParams,
  params: RequestParams = {},
) =>
  httpRequest<
    DomainResponse & {
      data?: V1StatConversationDistributionResp[];
    }
  >({
    path: `/api/v1/stat/conversation_distribution`,
    method: "GET",
    query: query,
    secure: true,
    type: ContentType.Json,
    format: "json",
    ...params,
  });

/**
 * @description 全局统计
 *
 * @tags stat
 * @name GetApiV1StatCount
 * @summary 全局统计
 * @request GET:/api/v1/stat/count
 * @secure
 * @response `200` `(DomainPWResponse & {
    data?: V1StatCountResp,

})` OK
 */

export const getApiV1StatCount = (
  query: GetApiV1StatCountParams,
  params: RequestParams = {},
) =>
  httpRequest<
    DomainPWResponse & {
      data?: V1StatCountResp;
    }
  >({
    path: `/api/v1/stat/count`,
    method: "GET",
    query: query,
    secure: true,
    type: ContentType.Json,
    format: "json",
    ...params,
  });

/**
 * @description 用户地理分布
 *
 * @tags stat
 * @name GetApiV1StatGeoCount
 * @summary 用户地理分布
 * @request GET:/api/v1/stat/geo_count
 * @secure
 * @response `200` `DomainResponse` OK
 */

export const getApiV1StatGeoCount = (
  query: GetApiV1StatGeoCountParams,
  params: RequestParams = {},
) =>
  httpRequest<DomainResponse>({
    path: `/api/v1/stat/geo_count`,
    method: "GET",
    query: query,
    secure: true,
    type: ContentType.Json,
    format: "json",
    ...params,
  });

/**
 * @description 热门文档
 *
 * @tags stat
 * @name GetApiV1StatHotPages
 * @summary 热门文档
 * @request GET:/api/v1/stat/hot_pages
 * @secure
 * @response `200` `(DomainResponse & {
    data?: (DomainHotPage)[],

})` OK
 */

export const getApiV1StatHotPages = (
  query: GetApiV1StatHotPagesParams,
  params: RequestParams = {},
) =>
  httpRequest<
    DomainResponse & {
      data?: DomainHotPage[];
    }
  >({
    path: `/api/v1/stat/hot_pages`,
    method: "GET",
    query: query,
    secure: true,
    type: ContentType.Json,
    format: "json",
    ...params,
  });

/**
 * @description GetInstantCount
 *
 * @tags stat
 * @name GetApiV1StatInstantCount
 * @summary GetInstantCount
 * @request GET:/api/v1/stat/instant_count
 * @secure
 * @response `200` `(DomainResponse & {
    data?: (DomainInstantCountResp)[],

})` OK
 */

export const getApiV1StatInstantCount = (
  query: GetApiV1StatInstantCountParams,
  params: RequestParams = {},
) =>
  httpRequest<
    DomainResponse & {
      data?: DomainInstantCountResp[];
    }
  >({
    path: `/api/v1/stat/instant_count`,
    method: "GET",
    query: query,
    secure: true,
    type: ContentType.Json,
    format: "json",
    ...params,
  });

/**
 * @description GetInstantPages
 *
 * @tags stat
 * @name GetApiV1StatInstantPages
 * @summary GetInstantPages
 * @request GET:/api/v1/stat/instant_pages
 * @secure
 * @response `200` `(DomainResponse & {
    data?: (DomainInstantPageResp)[],

})` OK
 */

export const getApiV1StatInstantPages = (
  query: GetApiV1StatInstantPagesParams,
  params: RequestParams = {},
) =>
  httpRequest<
    DomainResponse & {
      data?: DomainInstantPageResp[];
    }
  >({
    path: `/api/v1/stat/instant_pages`,
    method: "GET",
    query: query,
    secure: true,
    type: ContentType.Json,
    format: "json",
    ...params,
  });

/**
 * @description 来源域名
 *
 * @tags stat
 * @name GetApiV1StatRefererHosts
 * @summary 来源域名
 * @request GET:/api/v1/stat/referer_hosts
 * @secure
 * @response `200` `(DomainResponse & {
    data?: (DomainHotRefererHost)[],

})` OK
 */

export const getApiV1StatRefererHosts = (
  query: GetApiV1StatRefererHostsParams,
  params: RequestParams = {},
) =>
  httpRequest<
    DomainResponse & {
      data?: DomainHotRefererHost[];
    }
  >({
    path: `/api/v1/stat/referer_hosts`,
    method: "GET",
    query: query,
    secure: true,
    type: ContentType.Json,
    format: "json",
    ...params,
  });
