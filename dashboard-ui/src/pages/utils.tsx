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

import { ObjectMeta } from "kubernetes-types/meta/v1"
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import * as jsyaml from "js-yaml";

export interface Source {
    metadata?: ObjectMeta
}

export const formatData = (src: Source, format: string) => {
    src.metadata?.managedFields?.forEach((managedField) => {
        managedField.fieldsV1 = undefined
    })
    const data = format === 'json' ? JSON.stringify(src, null, "\t") : jsyaml.dump(src)
    return (
        <SyntaxHighlighter language={format} showLineNumbers >
            {data.trim()}
        </SyntaxHighlighter>
    )
}