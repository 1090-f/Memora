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
  DomainResponse,
  GetApiV1NodePermissionParams,
  V1NodePermissionEditReq,
  V1NodePermissionEditResp,
  V1NodePermissionResp,
} from "./types";

/**
 * @description 文档授权信息获取
 *
 * @tags NodePermission
 * @name GetApiV1NodePermission
 * @summary 文档授权信息获取
 * @request GET:/api/v1/node/permission
 * @secure
 * @response `200` `(DomainResponse & {
    data?: V1NodePermissionResp,

})` OK
 */

export const getApiV1NodePermission = (
  query: GetApiV1NodePermissionParams,
  params: RequestParams = {},
) =>
  httpRequest<
    DomainResponse & {
      data?: V1NodePermissionResp;
    }
  >({
    path: `/api/v1/node/permission`,
    method: "GET",
    query: query,
    secure: true,
    type: ContentType.Json,
    format: "json",
    ...params,
  });

/**
 * @description 文档授权信息更新
 *
 * @tags NodePermission
 * @name PatchApiV1NodePermissionEdit
 * @summary 文档授权信息更新
 * @request PATCH:/api/v1/node/permission/edit
 * @secure
 * @response `200` `(DomainResponse & {
    data?: V1NodePermissionEditResp,

})` OK
 */

export const patchApiV1NodePermissionEdit = (
  param: V1NodePermissionEditReq,
  params: RequestParams = {},
) =>
  httpRequest<
    DomainResponse & {
      data?: V1NodePermissionEditResp;
    }
  >({
    path: `/api/v1/node/permission/edit`,
    method: "PATCH",
    body: param,
    secure: true,
    type: ContentType.Json,
    format: "json",
    ...params,
  });
