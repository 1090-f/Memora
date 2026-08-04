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
  DeleteApiV1UserDeleteParams,
  DomainPWResponse,
  DomainResponse,
  V1CreateUserReq,
  V1CreateUserResp,
  V1LoginReq,
  V1LoginResp,
  V1ResetPasswordReq,
  V1UserInfoResp,
  V1UserListResp,
} from "./types";

/**
 * @description GetUser
 *
 * @tags user
 * @name GetApiV1User
 * @summary GetUser
 * @request GET:/api/v1/user
 * @response `200` `V1UserInfoResp` OK
 */

export const getApiV1User = (params: RequestParams = {}) =>
  httpRequest<V1UserInfoResp>({
    path: `/api/v1/user`,
    method: "GET",
    type: ContentType.Json,
    ...params,
  });

/**
 * @description CreateUser
 *
 * @tags user
 * @name PostApiV1UserCreate
 * @summary CreateUser
 * @request POST:/api/v1/user/create
 * @response `200` `(DomainResponse & {
    data?: V1CreateUserResp,

})` OK
 */

export const postApiV1UserCreate = (
  body: V1CreateUserReq,
  params: RequestParams = {},
) =>
  httpRequest<
    DomainResponse & {
      data?: V1CreateUserResp;
    }
  >({
    path: `/api/v1/user/create`,
    method: "POST",
    body: body,
    type: ContentType.Json,
    format: "json",
    ...params,
  });

/**
 * @description DeleteUser
 *
 * @tags user
 * @name DeleteApiV1UserDelete
 * @summary DeleteUser
 * @request DELETE:/api/v1/user/delete
 * @response `200` `DomainResponse` OK
 */

export const deleteApiV1UserDelete = (
  query: DeleteApiV1UserDeleteParams,
  params: RequestParams = {},
) =>
  httpRequest<DomainResponse>({
    path: `/api/v1/user/delete`,
    method: "DELETE",
    query: query,
    type: ContentType.Json,
    format: "json",
    ...params,
  });

/**
 * @description ListUsers
 *
 * @tags user
 * @name GetApiV1UserList
 * @summary ListUsers
 * @request GET:/api/v1/user/list
 * @response `200` `(DomainPWResponse & {
    data?: V1UserListResp,

})` OK
 */

export const getApiV1UserList = (params: RequestParams = {}) =>
  httpRequest<
    DomainPWResponse & {
      data?: V1UserListResp;
    }
  >({
    path: `/api/v1/user/list`,
    method: "GET",
    type: ContentType.Json,
    format: "json",
    ...params,
  });

/**
 * @description Login
 *
 * @tags user
 * @name PostApiV1UserLogin
 * @summary Login
 * @request POST:/api/v1/user/login
 * @response `200` `V1LoginResp` OK
 */

export const postApiV1UserLogin = (
  body: V1LoginReq,
  params: RequestParams = {},
) =>
  httpRequest<V1LoginResp>({
    path: `/api/v1/user/login`,
    method: "POST",
    body: body,
    type: ContentType.Json,
    format: "json",
    ...params,
  });

/**
 * @description ResetPassword
 *
 * @tags user
 * @name PutApiV1UserResetPassword
 * @summary ResetPassword
 * @request PUT:/api/v1/user/reset_password
 * @response `200` `DomainResponse` OK
 */

export const putApiV1UserResetPassword = (
  body: V1ResetPasswordReq,
  params: RequestParams = {},
) =>
  httpRequest<DomainResponse>({
    path: `/api/v1/user/reset_password`,
    method: "PUT",
    body: body,
    type: ContentType.Json,
    format: "json",
    ...params,
  });
