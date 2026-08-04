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
  DeleteApiV1AuthDeleteParams,
  DomainPWResponse,
  DomainResponse,
  GetApiV1AuthGetParams,
  GithubComChaitinPandaWikiApiAuthV1AuthGetResp,
  V1AuthSetReq,
} from "./types";

/**
 * @description 删除授权信息
 *
 * @tags Auth
 * @name DeleteApiV1AuthDelete
 * @summary 删除授权信息
 * @request DELETE:/api/v1/auth/delete
 * @secure
 * @response `200` `DomainResponse` OK
 */

export const deleteApiV1AuthDelete = (
  query: DeleteApiV1AuthDeleteParams,
  params: RequestParams = {},
) =>
  httpRequest<DomainResponse>({
    path: `/api/v1/auth/delete`,
    method: "DELETE",
    query: query,
    secure: true,
    type: ContentType.Json,
    format: "json",
    ...params,
  });

/**
 * @description 获取授权信息
 *
 * @tags Auth
 * @name GetApiV1AuthGet
 * @summary 获取授权信息
 * @request GET:/api/v1/auth/get
 * @secure
 * @response `200` `(DomainPWResponse & {
    data?: GithubComChaitinPandaWikiApiAuthV1AuthGetResp,

})` OK
 */

export const getApiV1AuthGet = (
  query: GetApiV1AuthGetParams,
  params: RequestParams = {},
) =>
  httpRequest<
    DomainPWResponse & {
      data?: GithubComChaitinPandaWikiApiAuthV1AuthGetResp;
    }
  >({
    path: `/api/v1/auth/get`,
    method: "GET",
    query: query,
    secure: true,
    type: ContentType.Json,
    format: "json",
    ...params,
  });

/**
 * @description 设置授权信息
 *
 * @tags Auth
 * @name PostApiV1AuthSet
 * @summary 设置授权信息
 * @request POST:/api/v1/auth/set
 * @secure
 * @response `200` `DomainResponse` OK
 */

export const postApiV1AuthSet = (
  param: V1AuthSetReq,
  params: RequestParams = {},
) =>
  httpRequest<DomainResponse>({
    path: `/api/v1/auth/set`,
    method: "POST",
    body: param,
    secure: true,
    type: ContentType.Json,
    format: "json",
    ...params,
  });
