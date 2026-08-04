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
import { DomainCompleteReq, DomainTextReq } from "./types";

/**
 * @description Tab-based document completion similar to AI coding's FIM (Fill in Middle)
 *
 * @tags creation
 * @name PostApiV1CreationTabComplete
 * @summary Tab-based document completion
 * @request POST:/api/v1/creation/tab-complete
 * @response `200` `string` success
 */

export const postApiV1CreationTabComplete = (
  body: DomainCompleteReq,
  params: RequestParams = {},
) =>
  httpRequest<string>({
    path: `/api/v1/creation/tab-complete`,
    method: "POST",
    body: body,
    type: ContentType.Json,
    format: "json",
    ...params,
  });

/**
 * @description Text creation
 *
 * @tags creation
 * @name PostApiV1CreationText
 * @summary Text creation
 * @request POST:/api/v1/creation/text
 * @response `200` `string` success
 */

export const postApiV1CreationText = (
  body: DomainTextReq,
  params: RequestParams = {},
) =>
  httpRequest<string>({
    path: `/api/v1/creation/text`,
    method: "POST",
    body: body,
    type: ContentType.Json,
    format: "json",
    ...params,
  });
