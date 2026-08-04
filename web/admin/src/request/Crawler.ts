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
  DomainPWResponse,
  V1CrawlerExportReq,
  V1CrawlerExportResp,
  V1CrawlerParseReq,
  V1CrawlerParseResp,
  V1CrawlerResultReq,
  V1CrawlerResultResp,
  V1CrawlerResultsReq,
  V1CrawlerResultsResp,
} from "./types";

/**
 * @description CrawlerExport
 *
 * @tags crawler
 * @name PostApiV1CrawlerExport
 * @summary CrawlerExport
 * @request POST:/api/v1/crawler/export
 * @response `200` `(DomainPWResponse & {
    data?: V1CrawlerExportResp,

})` OK
 */

export const postApiV1CrawlerExport = (
  body: V1CrawlerExportReq,
  params: RequestParams = {},
) =>
  httpRequest<
    DomainPWResponse & {
      data?: V1CrawlerExportResp;
    }
  >({
    path: `/api/v1/crawler/export`,
    method: "POST",
    body: body,
    type: ContentType.Json,
    format: "json",
    ...params,
  });

/**
 * @description 解析文档树
 *
 * @tags crawler
 * @name PostApiV1CrawlerParse
 * @summary 解析文档树
 * @request POST:/api/v1/crawler/parse
 * @response `200` `(DomainPWResponse & {
    data?: V1CrawlerParseResp,

})` OK
 */

export const postApiV1CrawlerParse = (
  body: V1CrawlerParseReq,
  params: RequestParams = {},
) =>
  httpRequest<
    DomainPWResponse & {
      data?: V1CrawlerParseResp;
    }
  >({
    path: `/api/v1/crawler/parse`,
    method: "POST",
    body: body,
    type: ContentType.Json,
    format: "json",
    ...params,
  });

/**
 * @description Retrieve the result of a previously started scraping task
 *
 * @tags crawler
 * @name GetApiV1CrawlerResult
 * @summary Get Crawler Result
 * @request GET:/api/v1/crawler/result
 * @response `200` `(DomainPWResponse & {
    data?: V1CrawlerResultResp,

})` OK
 */

export const getApiV1CrawlerResult = (
  body: V1CrawlerResultReq,
  params: RequestParams = {},
) =>
  httpRequest<
    DomainPWResponse & {
      data?: V1CrawlerResultResp;
    }
  >({
    path: `/api/v1/crawler/result`,
    method: "GET",
    body: body,
    type: ContentType.Json,
    format: "json",
    ...params,
  });

/**
 * @description Retrieve the results of a previously started scraping task
 *
 * @tags crawler
 * @name PostApiV1CrawlerResults
 * @summary Get Crawler Results
 * @request POST:/api/v1/crawler/results
 * @response `200` `(DomainPWResponse & {
    data?: V1CrawlerResultsResp,

})` OK
 */

export const postApiV1CrawlerResults = (
  param: V1CrawlerResultsReq,
  params: RequestParams = {},
) =>
  httpRequest<
    DomainPWResponse & {
      data?: V1CrawlerResultsResp;
    }
  >({
    path: `/api/v1/crawler/results`,
    method: "POST",
    body: param,
    type: ContentType.Json,
    format: "json",
    ...params,
  });
