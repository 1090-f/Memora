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
  DeleteApiV1AppParams,
  DomainAppDetailResp,
  DomainPWResponse,
  DomainResponse,
  DomainUpdateAppReq,
  GetApiV1AppDetailParams,
  PutApiV1AppParams,
} from "./types";

/**
 * @description Update app
 *
 * @tags app
 * @name PutApiV1App
 * @summary Update app
 * @request PUT:/api/v1/app
 * @secure
 * @response `200` `DomainResponse` OK
 */

export const putApiV1App = (
  query: PutApiV1AppParams,
  app: DomainUpdateAppReq,
  params: RequestParams = {},
) =>
  httpRequest<DomainResponse>({
    path: `/api/v1/app`,
    method: "PUT",
    query: query,
    body: app,
    secure: true,
    type: ContentType.Json,
    format: "json",
    ...params,
  });

/**
 * @description Delete app
 *
 * @tags app
 * @name DeleteApiV1App
 * @summary Delete app
 * @request DELETE:/api/v1/app
 * @secure
 * @response `200` `DomainResponse` OK
 */

export const deleteApiV1App = (
  query: DeleteApiV1AppParams,
  params: RequestParams = {},
) =>
  httpRequest<DomainResponse>({
    path: `/api/v1/app`,
    method: "DELETE",
    query: query,
    secure: true,
    type: ContentType.Json,
    ...params,
  });

/**
 * @description Get app detail
 *
 * @tags app
 * @name GetApiV1AppDetail
 * @summary Get app detail
 * @request GET:/api/v1/app/detail
 * @secure
 * @response `200` `(DomainPWResponse & {
    data?: DomainAppDetailResp,

})` OK
 */

export const getApiV1AppDetail = (
  query: GetApiV1AppDetailParams,
  params: RequestParams = {},
) =>
  httpRequest<
    DomainPWResponse & {
      data?: DomainAppDetailResp;
    }
  >({
    path: `/api/v1/app/detail`,
    method: "GET",
    query: query,
    secure: true,
    type: ContentType.Json,
    format: "json",
    ...params,
  });
