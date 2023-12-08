/*
 Copyright 2023 Juicedata Inc

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
 */

import { QuestionCircleOutlined } from '@ant-design/icons';
import { SelectLang as UmiSelectLang } from '@umijs/max';
import {
    disable as darkreaderDisable,
    enable as darkreaderEnable,
    setFetchMethod as setFetch,
} from '@umijs/ssr-darkreader';
import { Switch } from 'antd';

export const SelectLang = () => {
    return <UmiSelectLang style={{ padding: 4 }} />;
};

export const Question = () => {
    return (
        <div
            style={{
                display: 'flex',
                height: 26,
            }}
            onClick={() => {
                window.open('https://juicefs.com/docs/csi/introduction/');
            }}
        >
            <QuestionCircleOutlined />
        </div>
    );
};

const updateTheme = async (dark: boolean) => {
    if (typeof window === 'undefined') return;
    if (typeof window.MutationObserver === 'undefined') return;

    if (dark) {
        const defaultTheme = {
            brightness: 100,
            contrast: 90,
            sepia: 10,
        };

        const defaultFixes = {
            invert: [],
            css: '',
            ignoreInlineStyle: ['.react-switch-handle'],
            ignoreImageAnalysis: [],
            disableStyleSheetsProxy: true,
        };
        if (window.MutationObserver && window.fetch) {
            setFetch(window.fetch);
            darkreaderEnable(defaultTheme, defaultFixes);
        }
    } else {
        if (window.MutationObserver) darkreaderDisable();
    }
};

export const DarkMode = () => {
    return (
        <div>
            <Switch
                checkedChildren="🌛"
                unCheckedChildren="☀️"
                onClick={updateTheme}
                size="small"
            ></Switch>
        </div>
    );
};
